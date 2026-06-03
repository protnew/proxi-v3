package i18n

import "testing"

func TestTranslate(t *testing.T) {
	tr := NewTranslator("en")
	if got := tr.T("chat.send"); got != "Send" {
		t.Errorf("en chat.send = %q, want %q", got, "Send")
	}
	if got := tr.T("settings.title"); got != "Settings" {
		t.Errorf("en settings.title = %q, want %q", got, "Settings")
	}

	trRU := NewTranslator("ru")
	if got := trRU.T("chat.send"); got != "Отправить" {
		t.Errorf("ru chat.send = %q, want %q", got, "Отправить")
	}
	if got := trRU.T("login.welcome"); got != "Добро пожаловать" {
		t.Errorf("ru login.welcome = %q, want %q", got, "Добро пожаловать")
	}

	trZH := NewTranslator("zh")
	if got := trZH.T("chat.send"); got != "发送" {
		t.Errorf("zh chat.send = %q, want %q", got, "发送")
	}

	trES := NewTranslator("es")
	if got := trES.T("chat.send"); got != "Enviar" {
		t.Errorf("es chat.send = %q, want %q", got, "Enviar")
	}

	trAR := NewTranslator("ar")
	if got := trAR.T("chat.send"); got != "إرسال" {
		t.Errorf("ar chat.send = %q, want %q", got, "إرسال")
	}
}

func TestMissingKey(t *testing.T) {
	tr := NewTranslator("en")
	got := tr.T("nonexistent.key")
	want := "[nonexistent.key]"
	if got != want {
		t.Errorf("missing key = %q, want %q", got, want)
	}

	// Unknown language falls back to English, then to bracketed key
	trXX := NewTranslator("xx")
	got = trXX.T("chat.send")
	// Should fallback to English
	if got != "Send" {
		t.Errorf("unknown lang fallback = %q, want %q", got, "Send")
	}
	got = trXX.T("nonexistent.key")
	if got != "[nonexistent.key]" {
		t.Errorf("unknown lang missing key = %q, want %q", got, "[nonexistent.key]")
	}
}

func TestLoadBundle(t *testing.T) {
	LoadBundle("fr", map[string]string{
		"chat.send":     "Envoyer",
		"settings.title": "Paramètres",
		"login.create":   "Créer un compte",
	})

	tr := NewTranslator("fr")
	if got := tr.T("chat.send"); got != "Envoyer" {
		t.Errorf("fr chat.send = %q, want %q", got, "Envoyer")
	}
	if got := tr.T("settings.title"); got != "Paramètres" {
		t.Errorf("fr settings.title = %q, want %q", got, "Paramètres")
	}
	if got := tr.T("login.create"); got != "Créer un compte" {
		t.Errorf("fr login.create = %q, want %q", got, "Créer un compte")
	}
}

func TestAvailableLanguages(t *testing.T) {
	langs := AvailableLanguages()
	if len(langs) < 5 {
		t.Errorf("expected at least 5 languages, got %d", len(langs))
	}
}
