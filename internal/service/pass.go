package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"native-ios-passkit-wip/internal/applepass"
	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

type PassService struct {
	cfg          config.Config
	repo         MemberRepository
	walletPasses WalletPassRepository
	campaigns    ActiveCampaignRepository
	credentials  AppleWalletCredentialResolver
	designs      ApplePassDesignResolver
	linker       PassLinker
	timeout      time.Duration
}

type WalletPassRepository interface {
	GetOrCreate(ctx context.Context, input domain.CreateWalletPassInput) (domain.WalletPass, error)
	GetBySerial(ctx context.Context, passTypeIdentifier string, serialNumber string) (domain.WalletPass, error)
	TouchBySerial(ctx context.Context, serialNumber string) error
}

type ActiveCampaignRepository interface {
	ActiveCampaignForMember(ctx context.Context, memberID string) (domain.Campaign, error)
}

type PassPreview struct {
	Pass        applepass.Pass
	Member      applepass.StampCardMember
	Barcode     applepass.Barcode
	PassURL     string
	PreviewURL  string
	QRCodeURL   string
	LogoBase64  string
	StripBase64 string
}

func NewPassService(cfg config.Config, repo MemberRepository, linker PassLinker) PassService {
	return PassService{
		cfg:     cfg,
		repo:    repo,
		linker:  linker,
		timeout: 5 * time.Second,
	}
}

func NewPassServiceWithWalletPasses(cfg config.Config, repo MemberRepository, walletPasses WalletPassRepository, linker PassLinker) PassService {
	service := NewPassService(cfg, repo, linker)
	service.walletPasses = walletPasses
	return service
}

func NewPassServiceWithWalletPassesAndCampaigns(cfg config.Config, repo MemberRepository, walletPasses WalletPassRepository, campaigns ActiveCampaignRepository, linker PassLinker, credentialResolvers ...AppleWalletCredentialResolver) PassService {
	service := NewPassServiceWithWalletPasses(cfg, repo, walletPasses, linker)
	service.campaigns = campaigns
	if len(credentialResolvers) > 0 {
		service.credentials = credentialResolvers[0]
		if designs, ok := credentialResolvers[0].(ApplePassDesignResolver); ok {
			service.designs = designs
		}
	}
	return service
}

func (s PassService) ExampleLinks(memberID string) map[string]string {
	return map[string]string{
		"passURL":    s.linker.PassURL(memberID),
		"previewURL": s.linker.PreviewURL(memberID),
	}
}

func (s PassService) CreatePackage(ctx context.Context, memberID string) ([]byte, error) {
	return s.createPackage(ctx, memberID, "")
}

func (s PassService) createPackage(ctx context.Context, memberID string, expectedPassTypeIdentifier string) ([]byte, error) {
	member, err := s.StampCardMember(ctx, memberID)
	if err != nil {
		return nil, err
	}

	credential, err := s.activeCredential(ctx, member.BrandName)
	if err != nil {
		return nil, err
	}
	if expectedPassTypeIdentifier != "" && credential.PassTypeIdentifier != expectedPassTypeIdentifier {
		return nil, ErrUnauthorized
	}
	pass, design, err := s.stampCardPass(ctx, member, credential)
	if err != nil {
		return nil, err
	}
	dynamicAssets, err := s.stampStripAssets(design, member)
	if err != nil {
		return nil, fmt.Errorf("render stamp strip: %w", err)
	}
	if len(dynamicAssets) == 0 {
		pass.SuppressStripShine = false
	}
	generator := s.generatorFor(credential, design)
	generator.DynamicAssets = dynamicAssets
	data, err := generator.Create(pass)
	if err != nil {
		return nil, fmt.Errorf("create pass package: %w", err)
	}

	return data, nil
}

func (s PassService) Preview(ctx context.Context, memberID string) (PassPreview, error) {
	member, err := s.StampCardMember(ctx, memberID)
	if err != nil {
		return PassPreview{}, err
	}

	credential, err := s.activeCredential(ctx, member.BrandName)
	if err != nil {
		return PassPreview{}, err
	}
	pass, design, err := s.stampCardPass(ctx, member, credential)
	if err != nil {
		return PassPreview{}, err
	}
	barcode := applepass.Barcode{}
	if len(pass.Barcodes) > 0 {
		barcode = pass.Barcodes[0]
	}

	dynamicAssets, err := s.stampStripAssets(design, member)
	if err != nil {
		return PassPreview{}, fmt.Errorf("render stamp strip: %w", err)
	}
	if len(dynamicAssets) == 0 {
		pass.SuppressStripShine = false
	}
	stripBase64 := ""
	if strip := dynamicAssets["strip@2x.png"]; len(strip) > 0 {
		stripBase64 = base64.StdEncoding.EncodeToString(strip)
	}
	return PassPreview{
		Pass:        pass,
		Member:      member,
		Barcode:     barcode,
		PassURL:     s.linker.PassURL(memberID),
		PreviewURL:  s.linker.PreviewURL(memberID),
		QRCodeURL:   s.linker.QRCodeURL(memberID),
		LogoBase64:  s.previewLogo(design),
		StripBase64: stripBase64,
	}, nil
}

func (s PassService) QRCode(ctx context.Context, memberID string) ([]byte, error) {
	member, err := s.StampCardMember(ctx, memberID)
	if err != nil {
		return nil, err
	}

	png, err := qrcode.Encode(member.BarcodeID, qrcode.Medium, 320)
	if err != nil {
		return nil, fmt.Errorf("encode qr code: %w", err)
	}

	return png, nil
}

func (s PassService) StampCardMember(ctx context.Context, memberID string) (applepass.StampCardMember, error) {
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		return applepass.StampCardMember{}, ValidationError{Message: "member id is required"}
	}
	if s.repo == nil {
		return sampleStampCardMember(memberID), nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	member, err := s.repo.GetByID(ctx, memberID)
	if err != nil {
		return applepass.StampCardMember{}, err
	}

	stampCardMember := storeMemberToStampCard(member)
	if s.campaigns != nil {
		campaign, err := s.campaigns.ActiveCampaignForMember(ctx, member.MemberID)
		if err != nil {
			return applepass.StampCardMember{}, fmt.Errorf("load active campaign: %w", err)
		}
		stampCardMember.PromotionTitle = campaign.Title
		stampCardMember.Promotion = campaign.Message
	}

	return stampCardMember, nil
}

func (s PassService) AuthenticatedPackage(ctx context.Context, passTypeIdentifier string, serialNumber string, authorization string) ([]byte, error) {
	if err := s.AuthorizePass(ctx, passTypeIdentifier, serialNumber, authorization); err != nil {
		return nil, err
	}
	return s.createPackage(ctx, serialNumber, strings.TrimSpace(passTypeIdentifier))
}

func (s PassService) AuthorizePass(ctx context.Context, passTypeIdentifier string, serialNumber string, authorization string) error {
	if s.walletPasses == nil {
		return ErrRepositoryUnavailable
	}
	passTypeIdentifier = strings.TrimSpace(passTypeIdentifier)
	serialNumber = strings.TrimSpace(serialNumber)
	scheme, token, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	token = strings.TrimSpace(token)
	if passTypeIdentifier == "" || serialNumber == "" || !ok || scheme != "ApplePass" || token == "" {
		return ErrUnauthorized
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	walletPass, err := s.walletPasses.GetBySerial(ctx, passTypeIdentifier, serialNumber)
	if err != nil {
		if errors.Is(err, domain.ErrWalletPassNotFound) {
			return ErrUnauthorized
		}
		return fmt.Errorf("authorize wallet pass: %w", err)
	}
	if walletPass.AuthenticationToken != token {
		return ErrUnauthorized
	}
	return nil
}

func (s PassService) MarkPassUpdated(ctx context.Context, serialNumber string) error {
	if s.walletPasses == nil {
		return ErrRepositoryUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := s.walletPasses.TouchBySerial(ctx, serialNumber); err != nil {
		return fmt.Errorf("mark pass updated: %w", err)
	}
	return nil
}

func (s PassService) stampCardPass(ctx context.Context, member applepass.StampCardMember, credential domain.ActiveAppleWalletCredential) (applepass.Pass, domain.ApplePassDesign, error) {
	walletPass, err := s.ensureWalletPass(ctx, credential.PassTypeIdentifier, member.ID)
	if err != nil {
		return applepass.Pass{}, domain.ApplePassDesign{}, err
	}
	design, err := s.activeDesign(ctx, credential.ApplePassTypeID, member)
	if err != nil {
		return applepass.Pass{}, domain.ApplePassDesign{}, err
	}
	links := make([]applepass.StampCardLink, 0, len(design.Links))
	for _, link := range design.Links {
		links = append(links, applepass.StampCardLink{
			Key:   link.LinkType,
			Label: link.Label,
			URL:   link.URL,
		})
	}

	pass := applepass.NewStampCardPass(applepass.TemplateConfig{
		PassTypeID:          credential.PassTypeIdentifier,
		TeamID:              credential.TeamIdentifier,
		OrganizationName:    credential.OrganizationName,
		WebServiceURL:       s.appleWebServiceURL(),
		AuthenticationToken: walletPass.AuthenticationToken,
	}, applepass.StampCardDesign{
		ProgramName:         design.ProgramName,
		PassDescription:     design.PassDescription,
		StampGoal:           design.StampGoal,
		BackgroundColor:     design.BackgroundColor,
		ForegroundColor:     design.ForegroundColor,
		LabelColor:          design.LabelColor,
		StampInstructions:   design.StampInstructions,
		RewardTitle:         design.RewardTitle,
		RewardDetails:       design.RewardDetails,
		SupportText:         design.SupportText,
		TermsText:           design.TermsText,
		Links:               links,
		CampaignDisplayMode: design.CampaignDisplayMode,
		StampIconKey:        design.StampIconKey,
	}, member)
	return pass, design, nil
}

func (s PassService) ensureWalletPass(ctx context.Context, passTypeIdentifier string, serialNumber string) (domain.WalletPass, error) {
	if s.walletPasses == nil {
		return domain.WalletPass{}, nil
	}

	token, err := domain.NewWalletAuthenticationToken()
	if err != nil {
		return domain.WalletPass{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	walletPass, err := s.walletPasses.GetOrCreate(ctx, domain.CreateWalletPassInput{
		PassTypeIdentifier:  passTypeIdentifier,
		SerialNumber:        serialNumber,
		AuthenticationToken: token,
	})
	if err != nil {
		return domain.WalletPass{}, fmt.Errorf("ensure wallet pass: %w", err)
	}
	return walletPass, nil
}

func (s PassService) activeCredential(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error) {
	if s.credentials != nil {
		credential, err := s.credentials.ResolveActiveByMerchant(ctx, merchant)
		if err == nil {
			return credential, nil
		}
		if !errors.Is(err, domain.ErrAppleWalletIssuerNotFound) {
			return domain.ActiveAppleWalletCredential{}, err
		}
	}

	if strings.TrimSpace(s.cfg.PassTypeID) == "" || strings.TrimSpace(s.cfg.TeamID) == "" {
		return domain.ActiveAppleWalletCredential{}, domain.ErrAppleWalletNotProvisioned
	}
	return domain.ActiveAppleWalletCredential{
		OrganizationName:   s.cfg.OrganizationName,
		TeamIdentifier:     s.cfg.TeamID,
		PassTypeIdentifier: s.cfg.PassTypeID,
		PassStyle:          "storeCard",
		CertificateRef:     s.cfg.PassCertPath,
		PrivateKeyRef:      s.cfg.PassKeyPath,
	}, nil
}

func (s PassService) generatorFor(credential domain.ActiveAppleWalletCredential, design domain.ApplePassDesign) applepass.Generator {
	return applepass.Generator{
		SignerCertPath:  credential.CertificateRef,
		SignerKeyPath:   credential.PrivateKeyRef,
		WWDRCertPath:    s.cfg.WWDRCertPath,
		OpenSSLBin:      s.cfg.OpenSSLBin,
		AssetBundlePath: s.assetBundlePath(design),
	}
}

func (s PassService) activeDesign(ctx context.Context, applePassTypeID string, member applepass.StampCardMember) (domain.ApplePassDesign, error) {
	if s.designs != nil && strings.TrimSpace(applePassTypeID) != "" {
		design, err := s.designs.ResolveActivePassDesign(ctx, applePassTypeID)
		if err == nil {
			return design, nil
		}
		if !errors.Is(err, domain.ErrApplePassDesignNotFound) {
			return domain.ApplePassDesign{}, err
		}
	}
	programName := strings.TrimSpace(member.RewardName)
	if programName == "" {
		brandName := strings.TrimSpace(member.BrandName)
		if brandName == "" {
			programName = "Loyalty Card"
		} else {
			programName = brandName + " Loyalty Card"
		}
	}
	return domain.ApplePassDesign{
		ProgramName:         strings.TrimSpace(programName),
		PassDescription:     "Digital loyalty stamp card",
		StampGoal:           10,
		BackgroundColor:     "#5434F5",
		ForegroundColor:     "#FFFFFF",
		LabelColor:          "#E1DCFF",
		StampInstructions:   "Present this pass when making an eligible purchase to collect a stamp.",
		RewardTitle:         "Loyalty Reward",
		RewardDetails:       "Complete your stamp card to receive the merchant reward.",
		SupportText:         "Contact the merchant for loyalty card assistance.",
		TermsText:           "Collect one stamp for each eligible purchase. Rewards are subject to outlet terms.",
		CampaignDisplayMode: "details",
		StampIconKey:        "logo",
		Status:              "active",
	}, nil
}

func (s PassService) stampStripAssets(design domain.ApplePassDesign, member applepass.StampCardMember) (map[string][]byte, error) {
	artworkPath := ""
	assetPath := s.assetBundlePath(design)
	if assetPath != "" {
		switch design.StampIconKey {
		case "logo":
			artworkPath = filepath.Join(assetPath, "icon@3x.png")
		case "custom":
			artworkPath = filepath.Join(assetPath, "stamp.png")
		}
	}
	return applepass.RenderStampStrips(applepass.StampStripOptions{
		IconKey:         design.StampIconKey,
		ArtworkPath:     artworkPath,
		StampCount:      member.StampCount,
		StampGoal:       design.StampGoal,
		BackgroundColor: design.BackgroundColor,
		ForegroundColor: design.ForegroundColor,
		LabelColor:      design.LabelColor,
	})
}

func (s PassService) assetBundlePath(design domain.ApplePassDesign) string {
	if strings.TrimSpace(design.AssetBundleRef) == "" || strings.TrimSpace(s.cfg.PassAssetBasePath) == "" {
		return ""
	}
	return filepath.Join(s.cfg.PassAssetBasePath, design.AssetBundleRef)
}

func (s PassService) previewLogo(design domain.ApplePassDesign) string {
	assetPath := s.assetBundlePath(design)
	if assetPath == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(assetPath, "logo@2x.png"))
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func (s PassService) appleWebServiceURL() string {
	if strings.TrimSpace(s.cfg.WebServiceURL) != "" {
		return strings.TrimRight(s.cfg.WebServiceURL, "/")
	}
	if s.walletPasses == nil {
		return ""
	}
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/api/v1/apple/passes"
}

func storeMemberToStampCard(member domain.StoreMember) applepass.StampCardMember {
	return applepass.StampCardMember{
		ID:               member.MemberID,
		FullName:         member.CustomerName,
		BrandName:        member.Merchant,
		StampCount:       member.StampProgress,
		StampGoal:        10,
		AvailableRewards: member.AvailableRewards,
		BarcodeID:        "stamp-card:" + member.MemberID,
	}
}

func sampleStampCardMember(memberID string) applepass.StampCardMember {
	return applepass.StampCardMember{
		ID:         memberID,
		FullName:   "Taylor Example",
		BrandName:  "Daily Brew",
		StampCount: 6,
		StampGoal:  10,
		BarcodeID:  "stamp-card:" + memberID,
	}
}
