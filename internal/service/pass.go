package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"native-ios-passkit-wip/internal/applepass"
	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

type PassService struct {
	cfg       config.Config
	generator applepass.Generator
	repo      MemberRepository
	linker    PassLinker
	timeout   time.Duration
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

	pass := s.stampCardPass(member)
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

	pass := s.stampCardPass(member)
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

	return storeMemberToStampCard(member), nil
}

func (s PassService) stampCardPass(member applepass.StampCardMember) applepass.Pass {
	return applepass.NewStampCardPass(applepass.TemplateConfig{
		PassTypeID:       s.cfg.PassTypeID,
		TeamID:           s.cfg.TeamID,
		OrganizationName: s.cfg.OrganizationName,
		WebServiceURL:    s.cfg.WebServiceURL,
	}, member)
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
