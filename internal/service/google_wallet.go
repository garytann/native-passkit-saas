package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/walletobjects/v1"
	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
	"native-ios-passkit-wip/internal/googlewallet"
)

type GoogleWalletRepository interface {
	ProgramByMerchantID(context.Context, string) (domain.LoyaltyProgram, error)
	ProgramByID(context.Context, string) (domain.LoyaltyProgram, error)
	ProgramByMerchantName(context.Context, string) (domain.LoyaltyProgram, error)
	EnsureProgram(context.Context, domain.Merchant, string) (domain.LoyaltyProgram, error)
	UpdateProgram(context.Context, domain.UpdateLoyaltyProgramInput) (domain.LoyaltyProgram, error)
	BumpProgramVersion(context.Context, string) (domain.LoyaltyProgram, error)
	SetProgramStatus(context.Context, string, string) error
	ClassByMerchantID(context.Context, string) (domain.GoogleWalletClass, error)
	ClassByProgramID(context.Context, string) (domain.GoogleWalletClass, error)
	ClassByID(context.Context, string) (domain.GoogleWalletClass, error)
	ActiveClassByMerchantName(context.Context, string) (domain.GoogleWalletClass, domain.LoyaltyProgram, error)
	UpsertClass(context.Context, domain.UpsertGoogleWalletClassInput) (domain.GoogleWalletClass, error)
	SetClassState(context.Context, string, string, string, string, int) error
	GetOrCreateObject(context.Context, domain.UpsertGoogleWalletObjectInput) (domain.GoogleWalletObject, error)
	ObjectByMemberID(context.Context, string) (domain.GoogleWalletObject, error)
	ObjectByImageToken(context.Context, string) (domain.GoogleWalletObject, error)
	ObjectsByClassID(context.Context, string) ([]domain.GoogleWalletObject, error)
	MarkObjectSynced(context.Context, string, domain.StoreMember, int) error
	MarkObjectError(context.Context, string, string) error
	EnqueueSync(context.Context, domain.EnqueueWalletSyncJobInput) error
	ClaimPendingSyncJobs(context.Context, int) ([]domain.WalletSyncJob, error)
	CompleteSyncJob(context.Context, string) error
	RetrySyncJob(context.Context, domain.WalletSyncJob, string) error
}

type googleServiceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

type GoogleWalletService struct {
	cfg       config.Config
	repo      GoogleWalletRepository
	members   MemberRepository
	merchants MerchantRepository
	campaigns ActiveCampaignRepository
	api       *walletobjects.Service
	account   googleServiceAccount
}

type GoogleWalletAssets struct {
	ProgramLogo []byte
	WideLogo    []byte
}

func NewGoogleWalletService(ctx context.Context, cfg config.Config, repo GoogleWalletRepository, members MemberRepository, merchants MerchantRepository, campaigns ActiveCampaignRepository) (GoogleWalletService, error) {
	service := GoogleWalletService{cfg: cfg, repo: repo, members: members, merchants: merchants, campaigns: campaigns}
	if !cfg.GoogleWalletEnabled {
		return service, nil
	}
	if strings.TrimSpace(cfg.GoogleWalletIssuerID) == "" || strings.TrimSpace(cfg.GoogleCredentialsPath) == "" {
		return service, errors.New("Google Wallet is enabled but issuer ID or credentials path is missing")
	}
	data, err := os.ReadFile(cfg.GoogleCredentialsPath)
	if err != nil {
		return service, fmt.Errorf("read Google Wallet credentials: %w", err)
	}
	if err := json.Unmarshal(data, &service.account); err != nil {
		return service, fmt.Errorf("parse Google Wallet credentials: %w", err)
	}
	if service.account.ClientEmail == "" || service.account.PrivateKey == "" {
		return service, errors.New("Google Wallet credentials are missing client_email or private_key")
	}
	service.api, err = walletobjects.NewService(ctx, option.WithCredentialsFile(cfg.GoogleCredentialsPath), option.WithScopes(walletobjects.WalletObjectIssuerScope))
	if err != nil {
		return service, fmt.Errorf("initialize Google Wallet API: %w", err)
	}
	return service, nil
}

func (s GoogleWalletService) Enabled() bool {
	return s.cfg.GoogleWalletEnabled && s.api != nil && s.repo != nil
}

func (s GoogleWalletService) Configuration(ctx context.Context, merchantID string) (domain.GoogleWalletConfiguration, error) {
	result := domain.GoogleWalletConfiguration{Enabled: s.Enabled()}
	if s.repo == nil {
		return result, ErrRepositoryUnavailable
	}
	program, err := s.repo.ProgramByMerchantID(ctx, strings.TrimSpace(merchantID))
	if err != nil && !errors.Is(err, domain.ErrLoyaltyProgramNotFound) {
		return result, err
	}
	if err == nil {
		result.Program = &program
		assetDirectory := filepath.Join(s.cfg.PassAssetBasePath, program.AssetBundleRef)
		_, logoErr := os.Stat(filepath.Join(assetDirectory, "google-program-logo.png"))
		result.ProgramLogoUploaded = logoErr == nil
		_, wideLogoErr := os.Stat(filepath.Join(assetDirectory, "google-wide-logo.png"))
		result.WideLogoUploaded = wideLogoErr == nil
	}
	class, err := s.repo.ClassByMerchantID(ctx, strings.TrimSpace(merchantID))
	if err != nil && !errors.Is(err, domain.ErrGoogleWalletClassNotFound) {
		return result, err
	}
	if err == nil {
		result.Class = &class
	}
	return result, nil
}

func (s GoogleWalletService) EnsureProgram(ctx context.Context, merchantID string) (domain.LoyaltyProgram, error) {
	if s.repo == nil || s.merchants == nil {
		return domain.LoyaltyProgram{}, ErrRepositoryUnavailable
	}
	merchant, err := s.merchants.GetByID(ctx, strings.TrimSpace(merchantID))
	if err != nil {
		return domain.LoyaltyProgram{}, err
	}
	programID, err := domain.NewLoyaltyProgramID()
	if err != nil {
		return domain.LoyaltyProgram{}, err
	}
	return s.repo.EnsureProgram(ctx, merchant, programID)
}

func (s GoogleWalletService) ProgramByMerchantID(ctx context.Context, merchantID string) (domain.LoyaltyProgram, error) {
	if s.repo == nil {
		return domain.LoyaltyProgram{}, ErrRepositoryUnavailable
	}
	return s.repo.ProgramByMerchantID(ctx, strings.TrimSpace(merchantID))
}

func (s GoogleWalletService) UpdateProgram(ctx context.Context, input domain.UpdateLoyaltyProgramInput) (domain.LoyaltyProgram, error) {
	if s.repo == nil {
		return domain.LoyaltyProgram{}, ErrRepositoryUnavailable
	}
	return s.repo.UpdateProgram(ctx, input)
}

func (s GoogleWalletService) StoreAssets(ctx context.Context, merchantID string, assets GoogleWalletAssets) (domain.GoogleWalletConfiguration, error) {
	program, err := s.EnsureProgram(ctx, merchantID)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	programLogo, err := normalizeGoogleProgramLogo(assets.ProgramLogo)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, ValidationError{Message: err.Error()}
	}
	var wideLogo []byte
	if len(assets.WideLogo) > 0 {
		wideLogo, err = normalizeGoogleWideLogo(assets.WideLogo)
		if err != nil {
			return domain.GoogleWalletConfiguration{}, ValidationError{Message: err.Error()}
		}
	}
	if strings.TrimSpace(program.AssetBundleRef) == "" {
		return domain.GoogleWalletConfiguration{}, ValidationError{Message: "loyalty program asset reference is missing"}
	}
	directory := filepath.Join(s.cfg.PassAssetBasePath, program.AssetBundleRef)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return domain.GoogleWalletConfiguration{}, fmt.Errorf("create Google Wallet asset directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "google-program-logo.png"), programLogo, 0o640); err != nil {
		return domain.GoogleWalletConfiguration{}, fmt.Errorf("store Google program logo: %w", err)
	}
	if len(wideLogo) > 0 {
		if err := os.WriteFile(filepath.Join(directory, "google-wide-logo.png"), wideLogo, 0o640); err != nil {
			return domain.GoogleWalletConfiguration{}, fmt.Errorf("store Google wide logo: %w", err)
		}
	}
	if _, err := s.repo.BumpProgramVersion(ctx, program.ProgramID); err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	if err := s.MarkProgramUpdated(ctx, merchantID); err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	return s.Configuration(ctx, merchantID)
}

func (s GoogleWalletService) Provision(ctx context.Context, merchantID string) (domain.GoogleWalletConfiguration, error) {
	if !s.Enabled() {
		return domain.GoogleWalletConfiguration{}, domain.ErrGoogleWalletDisabled
	}
	if err := validateGoogleWalletPublicBaseURL(s.cfg.PublicBaseURL); err != nil {
		return domain.GoogleWalletConfiguration{}, ValidationError{Message: err.Error()}
	}
	program, err := s.EnsureProgram(ctx, merchantID)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	merchant, err := s.merchants.GetByID(ctx, merchantID)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	logoPath := filepath.Join(s.cfg.PassAssetBasePath, program.AssetBundleRef, "google-program-logo.png")
	if _, err := os.Stat(logoPath); err != nil {
		return domain.GoogleWalletConfiguration{}, ValidationError{Message: "Google Wallet program logo must be uploaded before provisioning"}
	}
	classSuffix := "loyalty_" + domain.StableToken(merchant.MerchantID, 20)
	classID := s.cfg.GoogleWalletIssuerID + "." + classSuffix
	wideRef := ""
	if _, err := os.Stat(filepath.Join(s.cfg.PassAssetBasePath, program.AssetBundleRef, "google-wide-logo.png")); err == nil {
		wideRef = "google-wide-logo.png"
	}
	classResource := s.classResource(merchant, program, classID, "UNDER_REVIEW", wideRef != "")
	remote, err := s.upsertRemoteClass(ctx, classResource)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, fmt.Errorf("provision Google Wallet class: %w", err)
	}
	status := "pending"
	review := normalizedReviewStatus(remote.ReviewStatus)
	if _, err := s.repo.UpsertClass(ctx, domain.UpsertGoogleWalletClassInput{GoogleClassID: classID, ProgramID: program.ProgramID, IssuerID: s.cfg.GoogleWalletIssuerID, ClassSuffix: classSuffix, ReviewStatus: review, Status: status, ProgramLogoRef: "google-program-logo.png", WideLogoRef: wideRef}); err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	_ = s.repo.SetClassState(ctx, classID, review, status, "", program.Version)
	return s.Configuration(ctx, merchantID)
}

func (s GoogleWalletService) Activate(ctx context.Context, merchantID string) (domain.GoogleWalletConfiguration, error) {
	if !s.Enabled() {
		return domain.GoogleWalletConfiguration{}, domain.ErrGoogleWalletDisabled
	}
	class, err := s.repo.ClassByMerchantID(ctx, merchantID)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	remote, err := s.api.Loyaltyclass.Get(class.GoogleClassID).Context(ctx).Do()
	if err != nil {
		return domain.GoogleWalletConfiguration{}, fmt.Errorf("get Google Wallet class: %w", err)
	}
	review := normalizedReviewStatus(remote.ReviewStatus)
	if review != "APPROVED" {
		_ = s.repo.SetClassState(ctx, class.GoogleClassID, review, "pending", "Google class is not approved", class.SyncedProgramVersion)
		return domain.GoogleWalletConfiguration{}, ValidationError{Message: "Google Wallet class is not approved yet"}
	}
	program, err := s.repo.ProgramByMerchantID(ctx, merchantID)
	if err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	if err := s.repo.SetClassState(ctx, class.GoogleClassID, review, "active", "", program.Version); err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	if err := s.repo.SetProgramStatus(ctx, program.ProgramID, "active"); err != nil {
		return domain.GoogleWalletConfiguration{}, err
	}
	return s.Configuration(ctx, merchantID)
}

func (s GoogleWalletService) AvailableForMerchant(ctx context.Context, merchant string) bool {
	if !s.Enabled() {
		return false
	}
	_, _, err := s.repo.ActiveClassByMerchantName(ctx, merchant)
	return err == nil
}

func (s GoogleWalletService) StampGoalForMerchant(ctx context.Context, merchant string) (int, error) {
	if s.repo == nil {
		return 0, ErrRepositoryUnavailable
	}
	program, err := s.repo.ProgramByMerchantName(ctx, strings.TrimSpace(merchant))
	if err != nil {
		return 0, err
	}
	return program.StampGoal, nil
}

func (s GoogleWalletService) RewardTitleForMerchant(ctx context.Context, merchant string) (string, error) {
	if s.repo == nil {
		return "", ErrRepositoryUnavailable
	}
	program, err := s.repo.ProgramByMerchantName(ctx, strings.TrimSpace(merchant))
	if err != nil {
		return "", err
	}
	return program.RewardTitle, nil
}

func (s GoogleWalletService) RegistrationLink(memberID string) string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/api/v1/google-wallet/passes/" + url.PathEscape(memberID) + "/save"
}

func (s GoogleWalletService) SaveURL(ctx context.Context, memberID string) (string, error) {
	if !s.Enabled() {
		return "", domain.ErrGoogleWalletDisabled
	}
	member, err := s.members.GetByID(ctx, strings.TrimSpace(memberID))
	if err != nil {
		return "", err
	}
	class, program, err := s.repo.ActiveClassByMerchantName(ctx, member.Merchant)
	if err != nil {
		return "", err
	}
	objectID := s.cfg.GoogleWalletIssuerID + ".member_" + domain.StableToken(member.MemberID, 24)
	imageToken, err := domain.NewWalletAuthenticationToken()
	if err != nil {
		return "", err
	}
	object, err := s.repo.GetOrCreateObject(ctx, domain.UpsertGoogleWalletObjectInput{GoogleObjectID: objectID, MemberID: member.MemberID, GoogleClassID: class.GoogleClassID, ImageToken: imageToken})
	if err != nil {
		return "", err
	}
	resource := s.objectResource(member, program, object)
	if _, err := s.upsertRemoteObject(ctx, resource, false); err != nil {
		_ = s.repo.MarkObjectError(ctx, object.GoogleObjectID, err.Error())
		return "", fmt.Errorf("provision Google Wallet object: %w", err)
	}
	_ = s.repo.MarkObjectSynced(ctx, object.GoogleObjectID, member, program.Version)
	return s.signSaveURL(resource)
}

func (s GoogleWalletService) MarkPassUpdated(ctx context.Context, memberID string) error {
	if !s.Enabled() || s.repo == nil {
		return nil
	}
	object, err := s.repo.ObjectByMemberID(ctx, memberID)
	if errors.Is(err, domain.ErrGoogleWalletObjectNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	member, err := s.members.GetByID(ctx, memberID)
	if err != nil {
		return err
	}
	program, err := s.repo.ProgramByMerchantName(ctx, member.Merchant)
	if err != nil {
		return err
	}
	jobID, err := domain.NewWalletSyncJobID()
	if err != nil {
		return err
	}
	version := fmt.Sprintf("%d-%d-%d-%s", program.Version, member.StampProgress, member.AvailableRewards, jobID)
	return s.repo.EnqueueSync(ctx, domain.EnqueueWalletSyncJobInput{SyncJobID: jobID, Platform: "google", TargetType: "object", TargetID: object.MemberID, DesiredVersion: version})
}

func (s GoogleWalletService) MarkProgramUpdated(ctx context.Context, merchantID string) error {
	if !s.Enabled() {
		return nil
	}
	class, err := s.repo.ClassByMerchantID(ctx, merchantID)
	if errors.Is(err, domain.ErrGoogleWalletClassNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	program, err := s.repo.ProgramByMerchantID(ctx, merchantID)
	if err != nil {
		return err
	}
	jobID, err := domain.NewWalletSyncJobID()
	if err != nil {
		return err
	}
	return s.repo.EnqueueSync(ctx, domain.EnqueueWalletSyncJobInput{SyncJobID: jobID, Platform: "google", TargetType: "class", TargetID: class.GoogleClassID, DesiredVersion: fmt.Sprint(program.Version)})
}

func (s GoogleWalletService) NotifyCampaign(ctx context.Context, merchant string, campaign domain.Campaign) error {
	if !s.Enabled() {
		return nil
	}
	class, _, err := s.repo.ActiveClassByMerchantName(ctx, merchant)
	if err != nil {
		return err
	}
	request := &walletobjects.AddMessageRequest{Message: &walletobjects.Message{Id: "campaign_" + domain.StableToken(campaign.CampaignID, 16), Header: campaign.Title, Body: campaign.Message, MessageType: "TEXT_AND_NOTIFY"}}
	_, err = s.api.Loyaltyclass.Addmessage(class.GoogleClassID, request).Context(ctx).Do()
	if isGoogleQuotaExceeded(err) {
		request.Message.MessageType = "TEXT"
		_, err = s.api.Loyaltyclass.Addmessage(class.GoogleClassID, request).Context(ctx).Do()
	}
	if err != nil {
		return fmt.Errorf("send Google Wallet campaign message: %w", err)
	}
	return nil
}

func (s GoogleWalletService) SyncProgram(ctx context.Context, classID string) error {
	configurationClass, err := s.repo.ClassByID(ctx, classID)
	if err != nil {
		return err
	}
	program, err := s.repo.ProgramByID(ctx, configurationClass.ProgramID)
	if err != nil {
		return err
	}
	merchant, err := s.merchants.GetByID(ctx, program.MerchantID)
	if err != nil {
		return err
	}
	remote, err := s.upsertRemoteClass(ctx, s.classResource(merchant, program, configurationClass.GoogleClassID, configurationClass.ReviewStatus, configurationClass.WideLogoRef != ""))
	if err != nil {
		_ = s.repo.SetClassState(ctx, classID, configurationClass.ReviewStatus, "error", err.Error(), program.Version)
		return err
	}
	status := configurationClass.Status
	if status != "active" {
		status = "pending"
	}
	if err := s.repo.SetClassState(ctx, classID, normalizedReviewStatus(remote.ReviewStatus), status, "", program.Version); err != nil {
		return err
	}
	objects, err := s.repo.ObjectsByClassID(ctx, classID)
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := s.MarkPassUpdated(ctx, object.MemberID); err != nil {
			return err
		}
	}
	return nil
}

func (s GoogleWalletService) SyncMember(ctx context.Context, memberID string) error {
	object, err := s.repo.ObjectByMemberID(ctx, memberID)
	if err != nil {
		return err
	}
	member, err := s.members.GetByID(ctx, memberID)
	if err != nil {
		return err
	}
	program, err := s.repo.ProgramByMerchantName(ctx, member.Merchant)
	if err != nil {
		return err
	}
	if _, err := s.upsertRemoteObject(ctx, s.objectResource(member, program, object), true); err != nil {
		_ = s.repo.MarkObjectError(ctx, object.GoogleObjectID, err.Error())
		return err
	}
	return s.repo.MarkObjectSynced(ctx, object.GoogleObjectID, member, program.Version)
}

func (s GoogleWalletService) RunSyncWorker(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processSyncJobs(ctx)
		}
	}
}

func (s GoogleWalletService) processSyncJobs(ctx context.Context) {
	jobs, err := s.repo.ClaimPendingSyncJobs(ctx, 20)
	if err != nil {
		return
	}
	for _, job := range jobs {
		var err error
		if job.TargetType == "class" {
			err = s.SyncProgram(ctx, job.TargetID)
		} else {
			err = s.SyncMember(ctx, job.TargetID)
		}
		if err == nil {
			_ = s.repo.CompleteSyncJob(ctx, job.SyncJobID)
		} else {
			_ = s.repo.RetrySyncJob(ctx, job, err.Error())
		}
	}
}

func (s GoogleWalletService) StampImage(ctx context.Context, token string) ([]byte, error) {
	object, err := s.repo.ObjectByImageToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	member, err := s.members.GetByID(ctx, object.MemberID)
	if err != nil {
		return nil, err
	}
	program, err := s.repo.ProgramByMerchantName(ctx, member.Merchant)
	if err != nil {
		return nil, err
	}
	artwork := s.stampArtworkPath(program)
	if artwork == "" {
		return nil, nil
	}
	return googlewallet.RenderStampImage(googlewallet.StampImageOptions{ArtworkPath: artwork, StampCount: member.StampProgress, StampGoal: program.StampGoal, BackgroundColor: program.BackgroundColor})
}

func (s GoogleWalletService) Asset(ctx context.Context, programID, name string) ([]byte, error) {
	program, err := s.repo.ProgramByID(ctx, programID)
	if err != nil {
		return nil, err
	}
	allowed := map[string]string{"program-logo.png": "google-program-logo.png", "wide-logo.png": "google-wide-logo.png"}
	ref := allowed[name]
	if ref == "" {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(s.cfg.PassAssetBasePath, program.AssetBundleRef, filepath.Base(ref)))
}

func (s GoogleWalletService) classResource(merchant domain.Merchant, program domain.LoyaltyProgram, classID, review string, hasWide bool) *walletobjects.LoyaltyClass {
	resource := &walletobjects.LoyaltyClass{Id: classID, IssuerName: merchant.Merchant, ReviewStatus: review, ProgramName: program.ProgramName, AccountNameLabel: "Member", AccountIdLabel: "Member ID", HexBackgroundColor: program.BackgroundColor, ProgramLogo: googleImage(s.publicAssetURL(program, "program-logo.png"), merchant.Merchant+" logo"), TextModulesData: classTextModules(program), LinksModuleData: googleLinks(program.Links)}
	if hasWide {
		resource.WideProgramLogo = googleImage(s.publicAssetURL(program, "wide-logo.png"), merchant.Merchant+" wordmark")
	}
	return resource
}

func (s GoogleWalletService) objectResource(member domain.StoreMember, program domain.LoyaltyProgram, object domain.GoogleWalletObject) *walletobjects.LoyaltyObject {
	resource := &walletobjects.LoyaltyObject{Id: object.GoogleObjectID, ClassId: object.GoogleClassID, State: "ACTIVE", AccountName: member.CustomerName, AccountId: member.MemberID, Barcode: &walletobjects.Barcode{Type: "QR_CODE", Value: "stamp-card:" + member.MemberID, AlternateText: member.MemberID}, LoyaltyPoints: &walletobjects.LoyaltyPoints{Label: "Stamps", Balance: &walletobjects.LoyaltyPointsBalance{Int: int64(member.StampProgress), ForceSendFields: []string{"Int"}}}, SecondaryLoyaltyPoints: &walletobjects.LoyaltyPoints{Label: "Rewards", Balance: &walletobjects.LoyaltyPointsBalance{Int: int64(member.AvailableRewards), ForceSendFields: []string{"Int"}}}, TextModulesData: objectTextModules(member, program)}
	if program.StampGoal <= 10 && s.stampArtworkPath(program) != "" {
		resource.HeroImage = googleImage(s.publicStampURL(object, program, member), "Loyalty stamp progress")
	}
	return resource
}

func (s GoogleWalletService) stampArtworkPath(program domain.LoyaltyProgram) string {
	filename := ""
	switch program.StampIconKey {
	case "logo":
		filename = "google-program-logo.png"
	case "custom":
		filename = "stamp.png"
	default:
		return ""
	}
	path := filepath.Join(s.cfg.PassAssetBasePath, program.AssetBundleRef, filename)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func (s GoogleWalletService) upsertRemoteClass(ctx context.Context, resource *walletobjects.LoyaltyClass) (*walletobjects.LoyaltyClass, error) {
	existing, err := s.api.Loyaltyclass.Get(resource.Id).Context(ctx).Do()
	if isGoogleNotFound(err) {
		return s.api.Loyaltyclass.Insert(resource).Context(ctx).Do()
	}
	if err != nil {
		return nil, err
	}
	resource.ReviewStatus = existing.ReviewStatus
	if strings.EqualFold(existing.ReviewStatus, "APPROVED") {
		resource.ReviewStatus = "UNDER_REVIEW"
	}
	return s.api.Loyaltyclass.Update(resource.Id, resource).Context(ctx).Do()
}

func (s GoogleWalletService) upsertRemoteObject(ctx context.Context, resource *walletobjects.LoyaltyObject, notify bool) (*walletobjects.LoyaltyObject, error) {
	_, err := s.api.Loyaltyobject.Get(resource.Id).Context(ctx).Do()
	if isGoogleNotFound(err) {
		return s.api.Loyaltyobject.Insert(resource).Context(ctx).Do()
	}
	if err != nil {
		return nil, err
	}
	resource.NotifyPreference = "DO_NOT_NOTIFY"
	if notify {
		resource.NotifyPreference = "NOTIFY_ON_UPDATE"
	}
	updated, err := s.api.Loyaltyobject.Update(resource.Id, resource).Context(ctx).Do()
	if isGoogleQuotaExceeded(err) {
		resource.NotifyPreference = "DO_NOT_NOTIFY"
		return s.api.Loyaltyobject.Update(resource.Id, resource).Context(ctx).Do()
	}
	return updated, err
}

func (s GoogleWalletService) signSaveURL(object *walletobjects.LoyaltyObject) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(s.account.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("parse Google signing key: %w", err)
	}
	origin := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	if parsed, err := url.Parse(origin); err == nil {
		origin = parsed.Scheme + "://" + parsed.Host
	}
	minimal := &walletobjects.LoyaltyObject{Id: object.Id, ClassId: object.ClassId, State: "ACTIVE"}
	now := time.Now()
	claims := jwt.MapClaims{"iss": s.account.ClientEmail, "aud": "google", "typ": "savetowallet", "iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(), "origins": []string{origin}, "payload": map[string]any{"loyaltyObjects": []*walletobjects.LoyaltyObject{minimal}}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign Google Wallet save token: %w", err)
	}
	return "https://pay.google.com/gp/v/save/" + token, nil
}

func (s GoogleWalletService) publicAssetURL(program domain.LoyaltyProgram, name string) string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/api/v1/public/google-wallet/assets/" + url.PathEscape(program.ProgramID) + "/" + name + "?v=" + fmt.Sprint(program.Version)
}
func (s GoogleWalletService) publicStampURL(object domain.GoogleWalletObject, program domain.LoyaltyProgram, member domain.StoreMember) string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/api/v1/public/google-wallet/images/" + url.PathEscape(object.ImageToken) + "/stamps.png?v=" + fmt.Sprintf("%d-%d", program.Version, member.StampProgress)
}
func validateGoogleWalletPublicBaseURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("Google Wallet artwork requires a public HTTPS URL; set PUBLIC_BASE_URL to an HTTPS tunnel or deployed application domain")
	}
	hostname := strings.ToLower(parsed.Hostname())
	ip := net.ParseIP(hostname)
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || (ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified())) {
		return errors.New("Google Wallet cannot load artwork from localhost or a private address; set PUBLIC_BASE_URL to a public HTTPS tunnel or deployed application domain")
	}
	return nil
}
func googleImage(uri, description string) *walletobjects.Image {
	return &walletobjects.Image{SourceUri: &walletobjects.ImageUri{Uri: uri}, ContentDescription: localized(description)}
}
func localized(value string) *walletobjects.LocalizedString {
	return &walletobjects.LocalizedString{DefaultValue: &walletobjects.TranslatedString{Language: "en-SG", Value: value}}
}

func classTextModules(program domain.LoyaltyProgram) []*walletobjects.TextModuleData {
	return []*walletobjects.TextModuleData{{Id: "collect_stamps", Header: "How to collect stamps", Body: program.StampInstructions}, {Id: "reward_details", Header: "Reward details", Body: program.RewardDetails}, {Id: "support", Header: "Customer support", Body: program.SupportText}, {Id: "terms", Header: "Terms and conditions", Body: program.TermsText}, {Id: "powered_by", Header: "Powered by", Body: "Just Loyalty"}}
}
func objectTextModules(member domain.StoreMember, program domain.LoyaltyProgram) []*walletobjects.TextModuleData {
	remaining := program.StampGoal - member.StampProgress
	if remaining < 0 {
		remaining = 0
	}
	next := fmt.Sprintf("%d stamps away", remaining)
	if remaining == 1 {
		next = "1 stamp away"
	}
	if remaining == 0 {
		next = "Ready to claim"
	}
	return []*walletobjects.TextModuleData{{Id: "next_reward", Header: "Next reward", Body: next}, {Id: "reward_title", Header: "Reward", Body: program.RewardTitle}}
}
func googleLinks(links []domain.ApplePassLink) *walletobjects.LinksModuleData {
	result := &walletobjects.LinksModuleData{}
	for _, link := range links {
		result.Uris = append(result.Uris, &walletobjects.Uri{Id: link.LinkType, Uri: link.URL, Description: link.Label})
	}
	if len(result.Uris) == 0 {
		return nil
	}
	return result
}
func isGoogleNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound
}
func isGoogleQuotaExceeded(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Code == http.StatusTooManyRequests {
		return true
	}
	if apiErr.Code != http.StatusForbidden {
		return false
	}
	for _, detail := range apiErr.Errors {
		reason := strings.ToLower(detail.Reason)
		if strings.Contains(reason, "quota") || strings.Contains(reason, "ratelimit") || strings.Contains(reason, "dailylimit") {
			return true
		}
	}
	return false
}

func normalizedReviewStatus(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return "UNDER_REVIEW"
	}
	return value
}

func normalizeGoogleProgramLogo(data []byte) ([]byte, error) {
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("Google program logo must be a valid PNG")
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != bounds.Dy() || bounds.Dx() < 660 {
		return nil, errors.New("Google program logo must be square and at least 660 x 660 pixels")
	}
	return encodeSanitizedPNG(decoded)
}
func normalizeGoogleWideLogo(data []byte) ([]byte, error) {
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("Google wide logo must be a valid PNG")
	}
	bounds := decoded.Bounds()
	ratio := float64(bounds.Dx()) / float64(bounds.Dy())
	if bounds.Dx() < 1280 || bounds.Dy() < 400 || ratio < 2.8 || ratio > 3.5 {
		return nil, errors.New("Google wide logo must be at least 1280 x 400 pixels with an approximately 16:5 aspect ratio")
	}
	return encodeSanitizedPNG(decoded)
}

func encodeSanitizedPNG(source image.Image) ([]byte, error) {
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		return nil, errors.New("sanitize Google Wallet PNG")
	}
	return output.Bytes(), nil
}
