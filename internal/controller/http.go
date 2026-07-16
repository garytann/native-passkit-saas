package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"native-ios-passkit-wip/internal/domain"
	"native-ios-passkit-wip/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

var errRouteNotFound = errors.New("pass route not found")

type HealthController struct{}

func (HealthController) Check(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

type CustomerController struct {
	members service.MemberService
}

func NewCustomerController(members service.MemberService) CustomerController {
	return CustomerController{members: members}
}

type registerMemberRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Merchant string `json:"merchant"`
}

type registerMemberResponse struct {
	Member     domain.StoreMember `json:"member"`
	PassURL    string             `json:"pass_url"`
	PreviewURL string             `json:"preview_url"`
	QRCodeURL  string             `json:"qr_code_url"`
}

func (c CustomerController) RegisterMember(w http.ResponseWriter, r *http.Request) {
	var request registerMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := c.members.RegisterMember(r.Context(), service.RegisterMemberInput{
		Name:     request.Name,
		Email:    request.Email,
		Merchant: request.Merchant,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerMemberResponse{
		Member:     result.Member,
		PassURL:    result.PassURL,
		PreviewURL: result.PreviewURL,
		QRCodeURL:  result.QRCodeURL,
	})
}

type MerchantController struct {
	merchants service.MerchantService
}

func NewMerchantController(merchants service.MerchantService) MerchantController {
	return MerchantController{merchants: merchants}
}

type registerMerchantRequest struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Company       string `json:"company"`
	MerchantEmail string `json:"merchant_email"`
	Password      string `json:"password"`
}

type registerMerchantResponse struct {
	Merchant domain.Merchant `json:"merchant"`
}

func (c MerchantController) RegisterMerchant(w http.ResponseWriter, r *http.Request) {
	var request registerMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	merchant, err := c.merchants.RegisterMerchant(r.Context(), service.RegisterMerchantInput{
		FirstName:     request.FirstName,
		LastName:      request.LastName,
		Company:       request.Company,
		MerchantEmail: request.MerchantEmail,
		Password:      request.Password,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerMerchantResponse{
		Merchant: merchant,
	})
}

type PassController struct {
	passes     service.PassService
	pathPrefix string
}

func NewPassController(passes service.PassService, pathPrefix string) PassController {
	return PassController{
		passes:     passes,
		pathPrefix: pathPrefix,
	}
}

func (c PassController) Example(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, c.passes.ExampleLinks("member-1001"))
}

func (c PassController) Routes(w http.ResponseWriter, r *http.Request) {
	if _, ok := c.memberIDFromPreviewPath(r.URL.Path); ok {
		c.Preview(w, r)
		return
	}
	if _, ok := c.memberIDFromQRCodePath(r.URL.Path); ok {
		c.QRCode(w, r)
		return
	}
	c.Package(w, r)
}

func (c PassController) Package(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromPackagePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	data, err := c.passes.CreatePackage(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.pkpass")
	w.Header().Set("Content-Disposition", `attachment; filename="`+memberID+`.pkpass"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (c PassController) Preview(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromPreviewPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	data, err := c.passes.Preview(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	var out bytes.Buffer
	if err := previewTemplate.Execute(&out, data); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Bytes())
}

func (c PassController) QRCode(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromQRCodePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	png, err := c.passes.QRCode(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (c PassController) memberIDFromPackagePath(path string) (string, bool) {
	const suffix = ".pkpass"
	if !hasRouteShape(path, c.pathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.pathPrefix, suffix), true
}

func (c PassController) memberIDFromPreviewPath(path string) (string, bool) {
	const suffix = "/preview"
	if !hasRouteShape(path, c.pathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.pathPrefix, suffix), true
}

func (c PassController) memberIDFromQRCodePath(path string) (string, bool) {
	const suffix = "/qr.png"
	if !hasRouteShape(path, c.pathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.pathPrefix, suffix), true
}

func hasRouteShape(path string, prefix string, suffix string) bool {
	return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix)
}

func trimRouteValue(path string, prefix string, suffix string) string {
	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.TrimSpace(value)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validationErr service.ValidationError
	switch {
	case errors.Is(err, service.ErrRepositoryUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Error())
	case errors.Is(err, domain.ErrMerchantAlreadyExists):
		writeError(w, http.StatusConflict, "merchant or merchant_email already exists")
	case errors.Is(err, domain.ErrStoreMemberNotFound):
		writeError(w, http.StatusNotFound, "member not found")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

var previewTemplate = template.Must(template.New("pass-preview").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Member.ID}} Apple Wallet Preview</title>
  <style>
    :root {
      color-scheme: light;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f4f6f8;
      color: #12202f;
    }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 32px 16px;
    }
    main {
      width: min(420px, 100%);
    }
    .pass {
      background: {{.Pass.BackgroundColor}};
      color: {{.Pass.ForegroundColor}};
      border-radius: 18px;
      overflow: hidden;
      box-shadow: 0 22px 50px rgba(20, 35, 54, .24);
    }
    .top {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      padding: 22px 24px 12px;
      align-items: start;
    }
    .brand {
      font-weight: 700;
      font-size: 17px;
      line-height: 1.2;
    }
    .header-field {
      text-align: right;
    }
    .label {
      color: {{.Pass.LabelColor}};
      font-size: 10px;
      line-height: 1.2;
      text-transform: uppercase;
      font-weight: 700;
      letter-spacing: .04em;
    }
    .value {
      font-size: 18px;
      font-weight: 700;
      line-height: 1.2;
      margin-top: 3px;
    }
    .primary {
      padding: 22px 24px 24px;
    }
    .primary .value {
      font-size: 30px;
      font-weight: 750;
    }
    .grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 18px;
      padding: 0 24px 20px;
    }
    .stamp-board {
      margin: 0 24px 24px;
      padding: 16px;
      background: rgba(255, 255, 255, .14);
      border: 1px solid rgba(255, 255, 255, .22);
      border-radius: 14px;
    }
    .stamp-grid {
      display: grid;
      grid-template-columns: repeat(5, 1fr);
      gap: 10px;
      margin-top: 10px;
    }
    .stamp {
      aspect-ratio: 1;
      border-radius: 999px;
      display: grid;
      place-items: center;
      font-size: 13px;
      font-weight: 800;
      border: 2px solid rgba(255, 255, 255, .72);
      color: rgba(255, 255, 255, .82);
      background: rgba(255, 255, 255, .08);
    }
    .stamp.filled {
      background: #f6c860;
      border-color: #f6c860;
      color: #245d52;
      box-shadow: 0 6px 14px rgba(0, 0, 0, .16);
    }
    .barcode {
      background: #fff;
      color: #1d1d1f;
      margin: 0 24px 24px;
      border-radius: 10px;
      padding: 18px;
      text-align: center;
    }
    .qr {
      width: 176px;
      height: 176px;
      margin: 0 auto 10px;
      display: block;
    }
    .alt {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 13px;
      overflow-wrap: anywhere;
    }
    .meta {
      margin-top: 18px;
      font-size: 13px;
      line-height: 1.5;
      color: #4a5563;
    }
    .meta a {
      color: #195da8;
      overflow-wrap: anywhere;
    }
  </style>
</head>
<body>
  <main>
    <section class="pass" aria-label="Apple Wallet pass preview">
      <div class="top">
        <div class="brand">{{.Pass.LogoText}}</div>
        {{with index .Pass.StoreCard.HeaderFields 0}}
        <div class="header-field">
          <div class="label">{{.Label}}</div>
          <div class="value">{{.Value}}</div>
        </div>
        {{end}}
      </div>
      {{with index .Pass.StoreCard.PrimaryFields 0}}
      <div class="primary">
        <div class="label">{{.Label}}</div>
        <div class="value">{{.Value}}</div>
      </div>
      {{end}}
      <div class="grid">
        {{range .Pass.StoreCard.SecondaryFields}}
        <div>
          <div class="label">{{.Label}}</div>
          <div class="value">{{.Value}}</div>
        </div>
        {{end}}
        {{range .Pass.StoreCard.AuxiliaryFields}}
        <div>
          <div class="label">{{.Label}}</div>
          <div class="value">{{.Value}}</div>
        </div>
        {{end}}
      </div>
      <div class="stamp-board">
        <div class="label">{{.Member.StampCount}} of {{.Member.StampGoal}} stamps collected</div>
        <div class="stamp-grid" aria-label="Stamp progress">
          {{range .Stamps}}
          <div class="stamp{{if .Filled}} filled{{end}}">{{.Number}}</div>
          {{end}}
        </div>
      </div>
      <div class="barcode">
        <img class="qr" src="{{.QRCodeURL}}" alt="QR code for {{.Barcode.AltText}}">
        <div class="alt">{{.Barcode.AltText}}</div>
      </div>
    </section>
    <div class="meta">
      Preview only. Apple Wallet performs the final native rendering after a signed pass is opened on device.
      <br>
      Pass URL: <a href="{{.PassURL}}">{{.PassURL}}</a>
    </div>
  </main>
</body>
</html>`))
