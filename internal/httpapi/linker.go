package httpapi

import "strings"

type passLinker struct {
	apiBaseURL string
}

func newPassLinker(publicBaseURL string) passLinker {
	return passLinker{
		apiBaseURL: strings.TrimRight(publicBaseURL, "/") + APIV1Prefix,
	}
}

func (l passLinker) PassURL(memberID string) string {
	return l.apiBaseURL + "/apple/passes/" + memberID + ".pkpass"
}

func (l passLinker) PreviewURL(memberID string) string {
	return l.apiBaseURL + "/apple/passes/" + memberID + "/preview"
}

func (l passLinker) QRCodeURL(memberID string) string {
	return l.apiBaseURL + "/apple/passes/" + memberID + "/qr.png"
}
