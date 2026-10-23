package service

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/walletobjects/v1"
	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

func TestValidateGoogleWalletPublicBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "public HTTPS", value: "https://wallet-assets.example.com", wantErr: false},
		{name: "HTTPS tunnel", value: "https://example.ngrok-free.app", wantErr: false},
		{name: "localhost", value: "https://localhost", wantErr: true},
		{name: "loopback", value: "https://127.0.0.1", wantErr: true},
		{name: "private address", value: "https://192.168.1.20", wantErr: true},
		{name: "plain HTTP", value: "http://wallet-assets.example.com", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateGoogleWalletPublicBaseURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateGoogleWalletPublicBaseURL(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestGoogleWalletPublicURLsUsePublicBaseURL(t *testing.T) {
	wallet := GoogleWalletService{cfg: config.Config{PublicBaseURL: "https://app.just-loyalty.com/"}}
	program := domain.LoyaltyProgram{ProgramID: "LPR-TEST", Version: 4}
	object := domain.GoogleWalletObject{ImageToken: "image-token"}
	member := domain.StoreMember{StampProgress: 6}

	assetURL := wallet.publicAssetURL(program, "program-logo.png")
	wantAssetURL := "https://app.just-loyalty.com/api/v1/public/google-wallet/assets/LPR-TEST/program-logo.png?v=4"
	if assetURL != wantAssetURL {
		t.Fatalf("publicAssetURL() = %q, want %q", assetURL, wantAssetURL)
	}

	stampURL := wallet.publicStampURL(object, program, member)
	wantStampURL := "https://app.just-loyalty.com/api/v1/public/google-wallet/images/image-token/stamps.png?v=4-6"
	if stampURL != wantStampURL {
		t.Fatalf("publicStampURL() = %q, want %q", stampURL, wantStampURL)
	}
}

func TestNormalizeGoogleProgramLogo(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 660, 660))
	source.SetNRGBA(20, 20, color.NRGBA{R: 84, G: 52, B: 245, A: 120})
	normalized, err := normalizeGoogleProgramLogo(testPNGBytes(t, source))
	if err != nil {
		t.Fatalf("normalize program logo: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(normalized))
	if err != nil {
		t.Fatalf("decode normalized logo: %v", err)
	}
	if decoded.Bounds().Dx() != 660 || decoded.Bounds().Dy() != 660 {
		t.Fatalf("normalized dimensions = %v", decoded.Bounds())
	}
}

func TestNormalizeGoogleLogosRejectInvalidDimensions(t *testing.T) {
	if _, err := normalizeGoogleProgramLogo(testPNGBytes(t, image.NewRGBA(image.Rect(0, 0, 660, 640)))); err == nil {
		t.Fatal("expected non-square program logo to fail")
	}
	if _, err := normalizeGoogleWideLogo(testPNGBytes(t, image.NewRGBA(image.Rect(0, 0, 1200, 400)))); err == nil {
		t.Fatal("expected undersized wide logo to fail")
	}
	if _, err := normalizeGoogleWideLogo(testPNGBytes(t, image.NewRGBA(image.Rect(0, 0, 1280, 400)))); err != nil {
		t.Fatalf("expected 1280x400 wide logo to pass: %v", err)
	}
}

func TestGoogleCampaignTextModuleDisplayModes(t *testing.T) {
	campaign := domain.Campaign{CampaignID: "CMP-LATEST", Title: "Double Stamp Friday", Message: "Collect two stamps with every drink."}

	for _, mode := range []string{"details", "front_and_details", ""} {
		t.Run("includes promotion for "+mode, func(t *testing.T) {
			modules := classTextModules(domain.LoyaltyProgram{CampaignDisplayMode: mode}, campaign)
			promotion := findGoogleTextModule(modules, "latest_promotion")
			if promotion == nil {
				t.Fatal("expected latest promotion module")
			}
			if promotion.Header != "Latest Promotion" {
				t.Fatalf("promotion header = %q", promotion.Header)
			}
			if promotion.Body != "Double Stamp Friday\n\nCollect two stamps with every drink." {
				t.Fatalf("promotion body = %q", promotion.Body)
			}
		})
	}

	if module := findGoogleTextModule(classTextModules(domain.LoyaltyProgram{CampaignDisplayMode: "none"}, campaign), "latest_promotion"); module != nil {
		t.Fatal("campaign display mode none must omit the promotion module")
	}
	if module := findGoogleTextModule(classTextModules(domain.LoyaltyProgram{CampaignDisplayMode: "details"}, domain.Campaign{}), "latest_promotion"); module != nil {
		t.Fatal("an empty campaign must omit the promotion module")
	}
}

func TestMergeGoogleClassMessagesKeepsOnlyLatestCampaign(t *testing.T) {
	existing := []*walletobjects.Message{
		{Id: "welcome", Header: "Welcome"},
		{Id: "campaign_old", Header: "Old campaign"},
	}
	latest := []*walletobjects.Message{{Id: "campaign_latest", Header: "Latest campaign"}}

	merged := mergeGoogleClassMessages(existing, latest)
	if len(merged) != 2 {
		t.Fatalf("merged message count = %d, want 2", len(merged))
	}
	if merged[0].Id != "welcome" || merged[1].Id != "campaign_latest" {
		t.Fatalf("merged message IDs = %q, %q", merged[0].Id, merged[1].Id)
	}
}

func TestNotifyGoogleCampaignEnqueuesDurableSyncJob(t *testing.T) {
	repo := &googleWalletRepositoryStub{class: googleTestClass(), program: googleTestProgram()}
	service := GoogleWalletService{
		cfg:  config.Config{GoogleWalletEnabled: true},
		repo: repo,
		api:  &walletobjects.Service{},
	}
	campaign := googleTestCampaign("CMP-NEW")

	if err := service.NotifyCampaign(context.Background(), campaign.Merchant, campaign); err != nil {
		t.Fatalf("NotifyCampaign() error = %v", err)
	}
	if len(repo.enqueued) != 1 {
		t.Fatalf("enqueued jobs = %d, want 1", len(repo.enqueued))
	}
	job := repo.enqueued[0]
	if job.Platform != "google" || job.TargetType != "campaign" || job.TargetID != campaign.CampaignID || job.DesiredVersion != campaign.CampaignID {
		t.Fatalf("unexpected campaign sync job: %+v", job)
	}
}

func TestSyncGoogleCampaignUpdatesModuleAndIsIdempotent(t *testing.T) {
	campaign := googleTestCampaign("CMP-LATEST")
	remote := &walletobjects.LoyaltyClass{
		Id:           googleTestClass().GoogleClassID,
		ReviewStatus: "APPROVED",
		Messages: []*walletobjects.Message{
			{Id: "welcome", Header: "Welcome", Body: "Thanks for joining", MessageType: "TEXT"},
			{Id: "campaign_old", Header: "Old offer", Body: "Expired", MessageType: "TEXT"},
		},
	}
	apiState, api := newGoogleWalletAPITestServer(t, remote)
	repo := &googleWalletRepositoryStub{class: googleTestClass(), program: googleTestProgram()}
	campaigns := &googleCampaignRepositoryStub{campaign: campaign, latest: campaign}
	service := GoogleWalletService{cfg: googleTestConfig(), repo: repo, merchants: googleMerchantRepositoryStub{}, campaigns: campaigns, api: api}

	if err := service.SyncCampaign(context.Background(), campaign.CampaignID); err != nil {
		t.Fatalf("first SyncCampaign() error = %v", err)
	}
	if err := service.SyncCampaign(context.Background(), campaign.CampaignID); err != nil {
		t.Fatalf("replayed SyncCampaign() error = %v", err)
	}

	if len(apiState.addMessageTypes) != 1 || apiState.addMessageTypes[0] != "TEXT_AND_NOTIFY" {
		t.Fatalf("add message types = %v, want one TEXT_AND_NOTIFY", apiState.addMessageTypes)
	}
	promotion := findGoogleTextModule(apiState.class.TextModulesData, "latest_promotion")
	if promotion == nil || promotion.Body != googleCampaignBody(campaign) {
		t.Fatalf("latest promotion module = %+v", promotion)
	}
	if len(apiState.class.Messages) != 2 || apiState.class.Messages[0].Id != "welcome" || apiState.class.Messages[1].Id != googleCampaignMessageID(campaign.CampaignID) {
		t.Fatalf("remote messages = %+v", apiState.class.Messages)
	}
}

func TestSyncGoogleCampaignFallsBackWhenNotificationQuotaExceeded(t *testing.T) {
	campaign := googleTestCampaign("CMP-QUOTA")
	apiState, api := newGoogleWalletAPITestServer(t, &walletobjects.LoyaltyClass{Id: googleTestClass().GoogleClassID, ReviewStatus: "APPROVED"})
	apiState.quotaOnce = true
	program := googleTestProgram()
	program.CampaignDisplayMode = "none"
	repo := &googleWalletRepositoryStub{class: googleTestClass(), program: program}
	service := GoogleWalletService{cfg: googleTestConfig(), repo: repo, merchants: googleMerchantRepositoryStub{}, campaigns: &googleCampaignRepositoryStub{campaign: campaign, latest: campaign}, api: api}

	if err := service.SyncCampaign(context.Background(), campaign.CampaignID); err != nil {
		t.Fatalf("SyncCampaign() error = %v", err)
	}
	if len(apiState.addMessageTypes) != 2 || apiState.addMessageTypes[0] != "TEXT_AND_NOTIFY" || apiState.addMessageTypes[1] != "TEXT" {
		t.Fatalf("add message types = %v, want TEXT_AND_NOTIFY followed by TEXT", apiState.addMessageTypes)
	}
	if promotion := findGoogleTextModule(apiState.class.TextModulesData, "latest_promotion"); promotion != nil {
		t.Fatal("campaign display mode none must omit the module while retaining notification delivery")
	}
}

func TestSyncGoogleCampaignRetryDoesNotNotifyTwice(t *testing.T) {
	campaign := googleTestCampaign("CMP-RETRY")
	apiState, api := newGoogleWalletAPITestServer(t, &walletobjects.LoyaltyClass{Id: googleTestClass().GoogleClassID, ReviewStatus: "APPROVED"})
	apiState.failUpdateOnce = true
	repo := &googleWalletRepositoryStub{class: googleTestClass(), program: googleTestProgram()}
	service := GoogleWalletService{cfg: googleTestConfig(), repo: repo, merchants: googleMerchantRepositoryStub{}, campaigns: &googleCampaignRepositoryStub{campaign: campaign, latest: campaign}, api: api}

	if err := service.SyncCampaign(context.Background(), campaign.CampaignID); err == nil {
		t.Fatal("expected the first class update to fail")
	}
	if err := service.SyncCampaign(context.Background(), campaign.CampaignID); err != nil {
		t.Fatalf("retry SyncCampaign() error = %v", err)
	}
	if len(apiState.addMessageTypes) != 1 {
		t.Fatalf("notification attempts = %d, want 1", len(apiState.addMessageTypes))
	}
}

func TestSyncGoogleCampaignIgnoresDelayedOlderCampaign(t *testing.T) {
	requested := googleTestCampaign("CMP-OLD")
	latest := googleTestCampaign("CMP-NEW")
	apiState, api := newGoogleWalletAPITestServer(t, &walletobjects.LoyaltyClass{Id: googleTestClass().GoogleClassID, ReviewStatus: "APPROVED"})
	service := GoogleWalletService{
		cfg:       googleTestConfig(),
		repo:      &googleWalletRepositoryStub{class: googleTestClass(), program: googleTestProgram()},
		merchants: googleMerchantRepositoryStub{},
		campaigns: &googleCampaignRepositoryStub{campaign: requested, latest: latest},
		api:       api,
	}

	if err := service.SyncCampaign(context.Background(), requested.CampaignID); err != nil {
		t.Fatalf("SyncCampaign() error = %v", err)
	}
	if apiState.requestCount != 0 {
		t.Fatalf("Google API requests = %d, want 0 for stale campaign", apiState.requestCount)
	}
}

func TestGoogleWalletWorkerProcessesCampaignJobs(t *testing.T) {
	campaign := googleTestCampaign("CMP-WORKER")
	_, api := newGoogleWalletAPITestServer(t, &walletobjects.LoyaltyClass{Id: googleTestClass().GoogleClassID, ReviewStatus: "APPROVED"})
	repo := &googleWalletRepositoryStub{
		class:   googleTestClass(),
		program: googleTestProgram(),
		jobs: []domain.WalletSyncJob{{
			SyncJobID:  "WSJ-CAMPAIGN",
			TargetType: "campaign",
			TargetID:   campaign.CampaignID,
		}},
	}
	service := GoogleWalletService{cfg: googleTestConfig(), repo: repo, merchants: googleMerchantRepositoryStub{}, campaigns: &googleCampaignRepositoryStub{campaign: campaign, latest: campaign}, api: api}

	service.processSyncJobs(context.Background())

	if len(repo.completed) != 1 || repo.completed[0] != "WSJ-CAMPAIGN" {
		t.Fatalf("completed jobs = %v", repo.completed)
	}
	if len(repo.retried) != 0 {
		t.Fatalf("retried jobs = %v", repo.retried)
	}
}

func findGoogleTextModule(modules []*walletobjects.TextModuleData, id string) *walletobjects.TextModuleData {
	for _, module := range modules {
		if module != nil && module.Id == id {
			return module
		}
	}
	return nil
}

type googleWalletRepositoryStub struct {
	GoogleWalletRepository
	class     domain.GoogleWalletClass
	program   domain.LoyaltyProgram
	enqueued  []domain.EnqueueWalletSyncJobInput
	jobs      []domain.WalletSyncJob
	completed []string
	retried   []string
}

func (r *googleWalletRepositoryStub) ActiveClassByMerchantName(context.Context, string) (domain.GoogleWalletClass, domain.LoyaltyProgram, error) {
	return r.class, r.program, nil
}

func (r *googleWalletRepositoryStub) ClassByID(context.Context, string) (domain.GoogleWalletClass, error) {
	return r.class, nil
}

func (r *googleWalletRepositoryStub) ProgramByID(context.Context, string) (domain.LoyaltyProgram, error) {
	return r.program, nil
}

func (r *googleWalletRepositoryStub) SetClassState(context.Context, string, string, string, string, int) error {
	return nil
}

func (r *googleWalletRepositoryStub) EnqueueSync(_ context.Context, input domain.EnqueueWalletSyncJobInput) error {
	r.enqueued = append(r.enqueued, input)
	return nil
}

func (r *googleWalletRepositoryStub) ClaimPendingSyncJobs(context.Context, int) ([]domain.WalletSyncJob, error) {
	jobs := r.jobs
	r.jobs = nil
	return jobs, nil
}

func (r *googleWalletRepositoryStub) CompleteSyncJob(_ context.Context, syncJobID string) error {
	r.completed = append(r.completed, syncJobID)
	return nil
}

func (r *googleWalletRepositoryStub) RetrySyncJob(_ context.Context, job domain.WalletSyncJob, _ string) error {
	r.retried = append(r.retried, job.SyncJobID)
	return nil
}

type googleCampaignRepositoryStub struct {
	GoogleCampaignRepository
	campaign domain.Campaign
	latest   domain.Campaign
}

func (r *googleCampaignRepositoryStub) CampaignByID(context.Context, string) (domain.Campaign, error) {
	return r.campaign, nil
}

func (r *googleCampaignRepositoryStub) LatestActiveCampaignForMerchant(context.Context, string) (domain.Campaign, error) {
	return r.latest, nil
}

type googleMerchantRepositoryStub struct{ MerchantRepository }

func (googleMerchantRepositoryStub) GetByID(context.Context, string) (domain.Merchant, error) {
	return domain.Merchant{MerchantID: "MER-TEST", Merchant: "Test Coffee", MerchantSlug: "test-coffee"}, nil
}

type googleWalletAPIState struct {
	t               *testing.T
	class           *walletobjects.LoyaltyClass
	addMessageTypes []string
	quotaOnce       bool
	failUpdateOnce  bool
	requestCount    int
}

func newGoogleWalletAPITestServer(t *testing.T, class *walletobjects.LoyaltyClass) (*googleWalletAPIState, *walletobjects.Service) {
	t.Helper()
	state := &googleWalletAPIState{t: t, class: class}
	client := &http.Client{Transport: googleWalletRoundTripper(func(request *http.Request) (*http.Response, error) {
		response := httptest.NewRecorder()
		state.serveHTTP(response, request)
		return response.Result(), nil
	})}
	api, err := walletobjects.NewService(context.Background(), option.WithEndpoint("https://wallet.test/"), option.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("create test Google Wallet client: %v", err)
	}
	return state, api
}

type googleWalletRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip googleWalletRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func (s *googleWalletAPIState) serveHTTP(response http.ResponseWriter, request *http.Request) {
	s.requestCount++
	response.Header().Set("Content-Type", "application/json")
	if strings.HasSuffix(request.URL.Path, "/addMessage") {
		var body walletobjects.AddMessageRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			s.t.Errorf("decode add message request: %v", err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		messageType := ""
		if body.Message != nil {
			messageType = body.Message.MessageType
		}
		s.addMessageTypes = append(s.addMessageTypes, messageType)
		if s.quotaOnce {
			s.quotaOnce = false
			response.WriteHeader(http.StatusTooManyRequests)
			_, _ = response.Write([]byte(`{"error":{"code":429,"message":"notification quota exceeded"}}`))
			return
		}
		if body.Message != nil {
			s.class.Messages = append(s.class.Messages, body.Message)
		}
		_ = json.NewEncoder(response).Encode(s.class)
		return
	}

	switch request.Method {
	case http.MethodGet:
		_ = json.NewEncoder(response).Encode(s.class)
	case http.MethodPut:
		var updated walletobjects.LoyaltyClass
		if err := json.NewDecoder(request.Body).Decode(&updated); err != nil {
			s.t.Errorf("decode class update request: %v", err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		if s.failUpdateOnce {
			s.failUpdateOnce = false
			response.WriteHeader(http.StatusInternalServerError)
			_, _ = response.Write([]byte(`{"error":{"code":500,"message":"temporary failure"}}`))
			return
		}
		s.class = &updated
		_ = json.NewEncoder(response).Encode(s.class)
	default:
		response.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func googleTestConfig() config.Config {
	return config.Config{GoogleWalletEnabled: true, PublicBaseURL: "https://app.example.com"}
}

func googleTestProgram() domain.LoyaltyProgram {
	return domain.LoyaltyProgram{
		ProgramID:           "LPR-TEST",
		MerchantID:          "MER-TEST",
		ProgramName:         "Test Coffee Loyalty Card",
		PassDescription:     "Test Coffee Loyalty Card",
		StampGoal:           10,
		BackgroundColor:     "#5434F5",
		AssetBundleRef:      "test-coffee",
		StampInstructions:   "Buy a drink to collect a stamp.",
		RewardTitle:         "Free Coffee",
		RewardDetails:       "Complete ten stamps to earn a coffee.",
		SupportText:         "Contact Test Coffee for help.",
		TermsText:           "Merchant terms apply.",
		CampaignDisplayMode: "details",
		Version:             3,
	}
}

func googleTestClass() domain.GoogleWalletClass {
	return domain.GoogleWalletClass{
		GoogleClassID:  "3388000000000000000.test_coffee",
		ProgramID:      "LPR-TEST",
		ReviewStatus:   "APPROVED",
		Status:         "active",
		ProgramLogoRef: "google-program-logo.png",
	}
}

func googleTestCampaign(id string) domain.Campaign {
	return domain.Campaign{CampaignID: id, Merchant: "Test Coffee", Title: "Double Stamp Friday", Message: "Collect two stamps with every drink.", Status: "active"}
}

func testPNGBytes(t *testing.T, source image.Image) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	return output.Bytes()
}
