package httpapi

import (
	"bytes"
	"html/template"
	"net/http"

	qrcode "github.com/skip2/go-qrcode"

	"native-ios-passkit-wip/internal/applepass"
)

type passPreview struct {
	Pass       applepass.Pass
	Member     applepass.StampCardMember
	Barcode    applepass.Barcode
	Stamps     []stampPreview
	PassURL    string
	PreviewURL string
	QRCodeURL  string
}

type stampPreview struct {
	Number int
	Filled bool
}

func (s Server) handleApplePassPreview(w http.ResponseWriter, r *http.Request) {
	memberID, ok := memberIDFromPreviewPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound)
		return
	}

	member := sampleStampCardMember(memberID)
	pass := s.stampCardPass(member)

	barcode := applepass.Barcode{}
	if len(pass.Barcodes) > 0 {
		barcode = pass.Barcodes[0]
	}

	data := passPreview{
		Pass:       pass,
		Member:     member,
		Barcode:    barcode,
		Stamps:     stampPreviews(member.StampCount, member.StampGoal),
		PassURL:    s.cfg.PublicBaseURL + "/apple/passes/" + memberID + ".pkpass",
		PreviewURL: s.cfg.PublicBaseURL + "/apple/passes/" + memberID + "/preview",
		QRCodeURL:  s.cfg.PublicBaseURL + "/apple/passes/" + memberID + "/qr.png",
	}

	var out bytes.Buffer
	if err := previewTemplate.Execute(&out, data); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Bytes())
}

func (s Server) handleApplePassQRCode(w http.ResponseWriter, r *http.Request) {
	memberID, ok := memberIDFromQRCodePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound)
		return
	}

	member := sampleStampCardMember(memberID)
	png, err := qrcode.Encode(member.BarcodeID, qrcode.Medium, 320)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func memberIDFromPreviewPath(path string) (string, bool) {
	const prefix = "/apple/passes/"
	const suffix = "/preview"
	if !hasRouteShape(path, prefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, prefix, suffix), true
}

func memberIDFromQRCodePath(path string) (string, bool) {
	const prefix = "/apple/passes/"
	const suffix = "/qr.png"
	if !hasRouteShape(path, prefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, prefix, suffix), true
}

func stampPreviews(count int, goal int) []stampPreview {
	if goal < 0 {
		goal = 0
	}
	stamps := make([]stampPreview, 0, goal)
	for i := 1; i <= goal; i++ {
		stamps = append(stamps, stampPreview{
			Number: i,
			Filled: i <= count,
		})
	}
	return stamps
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
