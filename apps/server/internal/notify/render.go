package notify

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var (
	htmlLayout = htmltemplate.Must(htmltemplate.ParseFS(templateFS, "templates/layout.html.tmpl"))
	textLayout = template.Must(template.ParseFS(templateFS, "templates/layout.txt.tmpl"))
)

const footer = "Kamu menerima email ini karena kamu terlibat di sebuah pakta di Tepati. Koin di Tepati hanya catatan janji antar teman, bukan uang sungguhan."

// content is the copy for one email; the layouts add the greeting, button and footer.
type content struct {
	Name    string
	Subject string
	Heading string
	Lines   []string
	Action  string
	URL     string
	Footer  string
}

// Renderer writes the emails. baseURL is the public origin of the web app (APP_BASE_URL).
type Renderer struct{ baseURL string }

func NewRenderer(baseURL string) *Renderer {
	return &Renderer{baseURL: strings.TrimRight(baseURL, "/")}
}

func (r *Renderer) pactURL(id uuid.UUID) string { return r.baseURL + "/pacts/" + id.String() }

// Notification renders the email for one notification (an Immediate kind).
func (r *Renderer) Notification(n service.NotificationEmail) (Message, error) {
	c, ok := copyFor(n.Kind, n.PactTitle)
	if !ok {
		return Message{}, fmt.Errorf("notify: no email copy for kind %q", n.Kind)
	}
	c.URL = r.pactURL(n.PactID)
	return render(n.To, n.ToName, c)
}

// Digest renders the one email that covers a burst of Digest notifications.
func (r *Renderer) Digest(d service.DigestEmail) (Message, error) {
	if d.Kind != "proof_submitted" {
		return Message{}, fmt.Errorf("notify: no digest copy for kind %q", d.Kind)
	}
	c := content{
		Subject: fmt.Sprintf("%d bukti menunggu tinjauanmu di %s", d.Count, quote(d.PactTitle)),
		Heading: fmt.Sprintf("%d bukti baru dari rekanmu", d.Count),
		Lines: []string{
			fmt.Sprintf("Ada %d bukti menunggu tinjauanmu di pakta %s.", d.Count, quote(d.PactTitle)),
			"Tinjau sebelum batas waktunya habis. Kalau terlewat, bukti disetujui otomatis.",
		},
		Action: "Tinjau bukti",
		URL:    r.baseURL + "/review",
	}
	return render(d.To, d.ToName, c)
}

// Invite renders the invitation sent to an address that has no account yet.
func (r *Renderer) Invite(i service.InviteEmail) (Message, error) {
	c := content{
		Subject: fmt.Sprintf("%s mengajakmu membuat pakta belajar: %s", i.BackerName, i.PactTitle),
		Heading: fmt.Sprintf("%s mengajakmu membuat pakta", i.BackerName),
		Lines: []string{
			fmt.Sprintf("%s mengundangmu jadi pelaksana di pakta %s: kamu setor bukti belajar tiap hari sebelum batas waktu, dan kalian sepakat soal koin di awal.", i.BackerName, quote(i.PactTitle)),
			"Buka tautannya untuk membaca ketentuan lengkap. Kamu baru terikat setelah menandatanganinya.",
			"Tautan ini berlaku sampai " + indonesianDateTime(i.ExpiresAt) + " dan hanya bisa dipakai sekali.",
		},
		Action: "Baca ketentuan",
		URL:    r.baseURL + "/invite/" + url.PathEscape(i.Token),
	}
	return render(i.To, "", c)
}

func copyFor(kind, title string) (content, bool) {
	q := quote(title)
	switch kind {
	case "terms_changed":
		return content{
			Subject: "Ketentuan pakta " + q + " berubah",
			Heading: "Ketentuan pakta berubah",
			Lines:   []string{"Rekanmu mengubah ketentuan pakta " + q + ". Tanda tangan yang ada dibuka lagi, jadi kalian berdua perlu menandatangani ulang."},
			Action:  "Baca ketentuan baru",
		}, true
	case "terms_signed":
		return content{
			Subject: "Rekanmu menandatangani ketentuan " + q,
			Heading: "Rekanmu sudah menandatangani",
			Lines:   []string{"Rekanmu menyetujui ketentuan pakta " + q + ". Kalau kamu belum menandatangani, sekarang giliranmu."},
			Action:  "Buka pakta",
		}, true
	case "proof_rejected":
		return content{
			Subject: "Buktimu di " + q + " ditolak",
			Heading: "Buktimu ditolak",
			Lines:   []string{"Reviewer menolak buktimu untuk pakta " + q + ". Baca alasannya. Kalau menurutmu keliru, kamu bisa mengajukan sengketa sebelum batas waktunya habis."},
			Action:  "Baca alasannya",
		}, true
	case "proof_overridden":
		return content{
			Subject: "Persetujuan buktimu di " + q + " dibatalkan",
			Heading: "Persetujuan buktimu dibatalkan",
			Lines:   []string{"Backer membatalkan persetujuan otomatis atas buktimu di pakta " + q + ". Keputusan ini final dan tidak bisa disengketakan. Alasannya tercatat dan bisa kamu baca di pakta."},
			Action:  "Baca alasannya",
		}, true
	case "proof_auto_approved":
		return content{
			Subject: "Buktimu di " + q + " disetujui otomatis",
			Heading: "Buktimu disetujui otomatis",
			Lines:   []string{"Tidak ada yang meninjau buktimu sampai batas waktunya, jadi buktimu di pakta " + q + " disetujui otomatis. Backer masih bisa membatalkannya selama masa pembatalan."},
			Action:  "Buka pakta",
		}, true
	case "dispute_opened":
		return content{
			Subject: "Ada sengketa baru di " + q,
			Heading: "Ada sengketa baru",
			Lines:   []string{"Pelaksana tidak setuju dengan penolakan buktinya di pakta " + q + " dan mengajukan sengketa. Baca alasannya, lalu putuskan sebelum batas waktunya habis."},
			Action:  "Tinjau sengketa",
		}, true
	case "pact_settled":
		return content{
			Subject: "Pakta " + q + " selesai, ada payout yang perlu diurus",
			Heading: "Pakta selesai",
			Lines:   []string{"Pakta " + q + " selesai dan saldo akhir sudah dihitung. Cek pembagian koinnya dan tandai payout setelah dibayar. Aplikasi tidak memindahkan uang sungguhan."},
			Action:  "Lihat saldo akhir",
		}, true
	}
	return content{}, false
}

func render(to, name string, c content) (Message, error) {
	if strings.TrimSpace(name) == "" {
		name = "teman"
	}
	c.Name = name
	c.Footer = footer
	c.Subject = oneLine(c.Subject)
	var txt, htm bytes.Buffer
	if err := textLayout.Execute(&txt, c); err != nil {
		return Message{}, err
	}
	if err := htmlLayout.Execute(&htm, c); err != nil {
		return Message{}, err
	}
	return Message{To: to, ToName: name, Subject: c.Subject, Text: txt.String(), HTML: htm.String()}, nil
}

// quote wraps a user-supplied title in plain quotes; %q would add Go escapes.
func quote(s string) string { return "“" + s + "”" }

// oneLine keeps user-supplied text (a pact title) from breaking out of the Subject header.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

var months = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// indonesianDateTime formats in WIB, the zone every current user and pact is in.
func indonesianDateTime(t time.Time) string {
	wib := t.In(time.FixedZone("WIB", 7*3600))
	return fmt.Sprintf("%d %s %d pukul %02d.%02d WIB", wib.Day(), months[wib.Month()-1], wib.Year(), wib.Hour(), wib.Minute())
}
