package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"native-ios-passkit-wip/internal/applepass"
	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

type PassService struct {
	cfg          config.Config
	generator    applepass.Generator
	repo         MemberRepository
	walletPasses WalletPassRepository
	campaigns    ActiveCampaignRepository
	linker       PassLinker
	timeout      time.Duration
}

type WalletPassRepository interface {
	GetOrCreate(ctx context.Context, input domain.CreateWalletPassInput) (domain.WalletPass, error)
	GetBySerial(ctx context.Context, passTypeIdentifier string, serialNumber string) (domain.WalletPass, error)
	Touch(ctx context.Context, passTypeIdentifier string, serialNumber string) error
}

type ActiveCampaignRepository interface {
	ActiveCampaignForMember(ctx context.Context, memberID string) (domain.Campaign, error)
}

type PassPreview struct {
	Pass       applepass.Pass
	Member     applepass.StampCardMember
	Barcode    applepass.Barcode
	PassURL    string
	PreviewURL string
	QRCodeURL  string
}

func NewPassService(cfg config.Config, repo MemberRepository, linker PassLinker) PassService {
	return PassService{
		cfg: cfg,
		generator: applepass.Generator{
			SignerCertPath: cfg.PassCertPath,
			SignerKeyPath:  cfg.PassKeyPath,
			WWDRCertPath:   cfg.WWDRCertPath,
			OpenSSLBin:     cfg.OpenSSLBin,
		},
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

func NewPassServiceWithWalletPassesAndCampaigns(cfg config.Config, repo MemberRepository, walletPasses WalletPassRepository, campaigns ActiveCampaignRepository, linker PassLinker) PassService {
	service := NewPassServiceWithWalletPasses(cfg, repo, walletPasses, linker)
	service.campaigns = campaigns
	return service
}

func (s PassService) ExampleLinks(memberID string) map[string]string {
	return map[string]string{
		"passURL":    s.linker.PassURL(memberID),
		"previewURL": s.linker.PreviewURL(memberID),
	}
}

func (s PassService) CreatePackage(ctx context.Context, memberID string) ([]byte, error) {
	member, err := s.StampCardMember(ctx, memberID)
	if err != nil {
		return nil, err
	}

	pass, err := s.stampCardPass(ctx, member)
	if err != nil {
		return nil, err
	}
	data, err := s.generator.Create(pass)
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

	pass, err := s.stampCardPass(ctx, member)
	if err != nil {
		return PassPreview{}, err
	}
	barcode := applepass.Barcode{}
	if len(pass.Barcodes) > 0 {
		barcode = pass.Barcodes[0]
	}

	return PassPreview{
		Pass:       pass,
		Member:     member,
		Barcode:    barcode,
		PassURL:    s.linker.PassURL(memberID),
		PreviewURL: s.linker.PreviewURL(memberID),
		QRCodeURL:  s.linker.QRCodeURL(memberID),
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
		stampCardMember.Promotion = campaign.Message
	}

	return stampCardMember, nil
}

func (s PassService) AuthenticatedPackage(ctx context.Context, passTypeIdentifier string, serialNumber string, authorization string) ([]byte, error) {
	if err := s.AuthorizePass(ctx, passTypeIdentifier, serialNumber, authorization); err != nil {
		return nil, err
	}
	return s.CreatePackage(ctx, serialNumber)
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
	if err := s.walletPasses.Touch(ctx, s.cfg.PassTypeID, serialNumber); err != nil {
		return fmt.Errorf("mark pass updated: %w", err)
	}
	return nil
}

func (s PassService) stampCardPass(ctx context.Context, member applepass.StampCardMember) (applepass.Pass, error) {
	walletPass, err := s.ensureWalletPass(ctx, member.ID)
	if err != nil {
		return applepass.Pass{}, err
	}

	return applepass.NewStampCardPass(applepass.TemplateConfig{
		PassTypeID:          s.cfg.PassTypeID,
		TeamID:              s.cfg.TeamID,
		OrganizationName:    s.cfg.OrganizationName,
		WebServiceURL:       s.appleWebServiceURL(),
		AuthenticationToken: walletPass.AuthenticationToken,
	}, member), nil
}

func (s PassService) ensureWalletPass(ctx context.Context, serialNumber string) (domain.WalletPass, error) {
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
		PassTypeIdentifier:  s.cfg.PassTypeID,
		SerialNumber:        serialNumber,
		AuthenticationToken: token,
	})
	if err != nil {
		return domain.WalletPass{}, fmt.Errorf("ensure wallet pass: %w", err)
	}
	return walletPass, nil
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
		ID:         member.MemberID,
		FullName:   member.CustomerName,
		BrandName:  member.Merchant,
		StampCount: member.StampProgress,
		StampGoal:  10,
		BarcodeID:  "stamp-card:" + member.MemberID,
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
