// Package actions constructs immutable, reviewable payloads. Remote content is data, never consent.
package actions

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ans-4175/sela/internal/accounts"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type AttachmentInput struct {
	Path string `json:"path"`
}
type MailInput struct {
	To          []string          `json:"to"`
	Cc          []string          `json:"cc,omitempty"`
	Bcc         []string          `json:"bcc,omitempty"`
	Subject     string            `json:"subject"`
	Body        string            `json:"body"`
	Attachments []AttachmentInput `json:"attachments,omitempty"`
}
type Attachment struct {
	Name   string `json:"name"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Mail struct {
	From        string       `json:"from"`
	To          []string     `json:"to"`
	Cc          []string     `json:"cc,omitempty"`
	Bcc         []string     `json:"bcc,omitempty"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Raw         string       `json:"raw_base64url"`
	RawSHA256   string       `json:"raw_sha256"`
}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ValidateAddress(s string) bool {
	a, e := mail.ParseAddress(s)
	return e == nil && a.Address == s && !strings.ContainsAny(s, "\r\n\x00")
}
func BuildMail(a accounts.Account, in MailInput) (Mail, error) {
	m := Mail{From: a.Email, To: in.To, Cc: in.Cc, Bcc: in.Bcc, Subject: in.Subject, Body: in.Body}
	if a.Validate() != nil || len(in.To)+len(in.Cc)+len(in.Bcc) == 0 || len(in.To)+len(in.Cc)+len(in.Bcc) > 100 || len(in.Subject) > 998 || strings.ContainsAny(in.Subject, "\r\n\x00") || !utf8.ValidString(in.Body) || len(in.Body) > 1<<20 || len(in.Attachments) > 10 {
		return m, errors.New("invalid mail payload/size")
	}
	for _, list := range [][]string{in.To, in.Cc, in.Bcc} {
		for _, v := range list {
			if !ValidateAddress(v) {
				return m, errors.New("recipients must be plain email addresses")
			}
		}
	}
	random := make([]byte, 16)
	if _, e := rand.Read(random); e != nil {
		return m, errors.New("cannot generate message ID")
	}
	var raw bytes.Buffer
	fmt.Fprintf(&raw, "From: %s\r\n", a.Email)
	for _, h := range []struct {
		Name   string
		Values []string
	}{{"To", in.To}, {"Cc", in.Cc}, {"Bcc", in.Bcc}} {
		if len(h.Values) > 0 {
			fmt.Fprintf(&raw, "%s: %s\r\n", h.Name, strings.Join(h.Values, ", "))
		}
	}
	fmt.Fprintf(&raw, "Subject: %s\r\nMessage-ID: <%s@sela.local>\r\nMIME-Version: 1.0\r\n", mime.QEncoding.Encode("utf-8", in.Subject), hex.EncodeToString(random))
	var parts bytes.Buffer
	writer := multipart.NewWriter(&parts)
	fmt.Fprintf(&raw, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", writer.Boundary())
	text, e := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}, "Content-Transfer-Encoding": {"base64"}})
	if e != nil {
		return m, errors.New("cannot encode mail")
	}
	fmt.Fprint(text, foldBase64([]byte(in.Body)))
	total := len(in.Body)
	for _, file := range in.Attachments {
		f, e := os.Open(file.Path)
		if e != nil {
			return m, errors.New("cannot read attachment")
		}
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			f.Close()
			return m, errors.New("attachment must be a regular file <=8 MiB")
		}
		data := make([]byte, info.Size()+1)
		n, e := f.ReadAt(data, 0)
		f.Close()
		data = data[:n]
		if n != int(info.Size()) {
			return m, errors.New("attachment changed while reading")
		}
		total += n
		if total > 10<<20 {
			return m, errors.New("mail attachment total exceeds 10 MiB")
		}
		name := filepath.Base(file.Path)
		if strings.ContainsAny(name, "\r\n\x00") {
			return m, errors.New("invalid attachment name")
		}
		disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
		part, e := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}, "Content-Disposition": {disposition}, "Content-Transfer-Encoding": {"base64"}})
		if e != nil {
			return m, errors.New("cannot encode attachment")
		}
		fmt.Fprint(part, foldBase64(data))
		m.Attachments = append(m.Attachments, Attachment{Name: name, Bytes: n, SHA256: hash(data)})
	}
	if writer.Close() != nil {
		return m, errors.New("cannot finalize mail")
	}
	raw.Write(parts.Bytes())
	m.Raw = base64.RawURLEncoding.EncodeToString(raw.Bytes())
	m.RawSHA256 = hash(raw.Bytes())
	return m, nil
}
func foldBase64(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76])
		b.WriteString("\r\n")
		s = s[76:]
	}
	b.WriteString(s)
	b.WriteString("\r\n")
	return b.String()
}
func (m Mail) Validate(a accounts.Account) error {
	raw, e := base64.RawURLEncoding.DecodeString(m.Raw)
	if e != nil || hash(raw) != m.RawSHA256 || m.From != a.Email || len(raw) == 0 {
		return errors.New("mail snapshot integrity/account mismatch")
	}
	return nil
}
