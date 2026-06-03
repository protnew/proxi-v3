package i18n

import "fmt"

// Translator provides internationalization support for the messenger.
type Translator struct {
	lang    string
	bundles map[string]map[string]string
}

// bundles holds all built-in language bundles.
var bundles = map[string]map[string]string{}

func init() {
	// English
	LoadBundle("en", map[string]string{
		"chat.send":         "Send",
		"chat.receive":      "Receive",
		"chat.reply":        "Reply",
		"chat.forward":      "Forward",
		"chat.delete":       "Delete",
		"chat.edit":         "Edit",
		"chat.search":       "Search messages",
		"chat.attach":       "Attach file",
		"settings.title":    "Settings",
		"settings.language": "Language",
		"settings.theme":    "Theme",
		"settings.profile":  "Profile",
		"settings.security": "Security",
		"settings.notify":   "Notifications",
		"login.create":      "Create account",
		"login.restore":     "Restore account",
		"login.seed":        "Enter seed phrase",
		"login.welcome":     "Welcome",
		"login.logout":      "Logout",
		"contacts.title":    "Contacts",
		"contacts.add":      "Add contact",
		"contacts.block":    "Block",
		"error.network":    "Network error",
		"error.auth":       "Authentication failed",
	})

	// Russian
	LoadBundle("ru", map[string]string{
		"chat.send":         "Отправить",
		"chat.receive":      "Получить",
		"chat.reply":        "Ответить",
		"chat.forward":      "Переслать",
		"chat.delete":       "Удалить",
		"chat.edit":         "Редактировать",
		"chat.search":       "Поиск сообщений",
		"chat.attach":       "Прикрепить файл",
		"settings.title":    "Настройки",
		"settings.language": "Язык",
		"settings.theme":    "Тема",
		"settings.profile":  "Профиль",
		"settings.security": "Безопасность",
		"settings.notify":   "Уведомления",
		"login.create":      "Создать аккаунт",
		"login.restore":     "Восстановить аккаунт",
		"login.seed":        "Введите сид-фразу",
		"login.welcome":     "Добро пожаловать",
		"login.logout":      "Выйти",
		"contacts.title":    "Контакты",
		"contacts.add":      "Добавить контакт",
		"contacts.block":    "Заблокировать",
		"error.network":    "Ошибка сети",
		"error.auth":       "Ошибка аутентификации",
	})

	// Chinese (Simplified)
	LoadBundle("zh", map[string]string{
		"chat.send":         "发送",
		"chat.receive":      "接收",
		"chat.reply":        "回复",
		"chat.forward":      "转发",
		"chat.delete":       "删除",
		"chat.edit":         "编辑",
		"chat.search":       "搜索消息",
		"chat.attach":       "附件",
		"settings.title":    "设置",
		"settings.language": "语言",
		"settings.theme":    "主题",
		"settings.profile":  "个人资料",
		"settings.security": "安全",
		"settings.notify":   "通知",
		"login.create":      "创建账户",
		"login.restore":     "恢复账户",
		"login.seed":        "输入助记词",
		"login.welcome":     "欢迎",
		"login.logout":      "退出",
		"contacts.title":    "联系人",
		"contacts.add":      "添加联系人",
		"contacts.block":    "拉黑",
		"error.network":    "网络错误",
		"error.auth":       "认证失败",
	})

	// Spanish
	LoadBundle("es", map[string]string{
		"chat.send":         "Enviar",
		"chat.receive":      "Recibir",
		"chat.reply":        "Responder",
		"chat.forward":      "Reenviar",
		"chat.delete":       "Eliminar",
		"chat.edit":         "Editar",
		"chat.search":       "Buscar mensajes",
		"chat.attach":       "Adjuntar archivo",
		"settings.title":    "Configuración",
		"settings.language": "Idioma",
		"settings.theme":    "Tema",
		"settings.profile":  "Perfil",
		"settings.security": "Seguridad",
		"settings.notify":   "Notificaciones",
		"login.create":      "Crear cuenta",
		"login.restore":     "Restaurar cuenta",
		"login.seed":        "Introducir frase semilla",
		"login.welcome":     "Bienvenido",
		"login.logout":      "Cerrar sesión",
		"contacts.title":    "Contactos",
		"contacts.add":      "Agregar contacto",
		"contacts.block":    "Bloquear",
		"error.network":    "Error de red",
		"error.auth":       "Error de autenticación",
	})

	// Arabic
	LoadBundle("ar", map[string]string{
		"chat.send":         "إرسال",
		"chat.receive":      "استقبال",
		"chat.reply":        "رد",
		"chat.forward":      "إعادة توجيه",
		"chat.delete":       "حذف",
		"chat.edit":         "تعديل",
		"chat.search":       "بحث في الرسائل",
		"chat.attach":       "إرفاق ملف",
		"settings.title":    "الإعدادات",
		"settings.language": "اللغة",
		"settings.theme":    "المظهر",
		"settings.profile":  "الملف الشخصي",
		"settings.security": "الأمان",
		"settings.notify":   "الإشعارات",
		"login.create":      "إنشاء حساب",
		"login.restore":     "استعادة الحساب",
		"login.seed":        "أدخل عبارة البذور",
		"login.welcome":     "مرحباً",
		"login.logout":      "تسجيل الخروج",
		"contacts.title":    "جهات الاتصال",
		"contacts.add":      "إضافة جهة اتصال",
		"contacts.block":    "حظر",
		"error.network":    "خطأ في الشبكة",
		"error.auth":       "فشل المصادقة",
	})
}

// NewTranslator creates a new Translator for the given language.
func NewTranslator(lang string) *Translator {
	return &Translator{
		lang:    lang,
		bundles: bundles,
	}
}

// T returns the translated string for the given key. If no translation is
// found, it returns the key itself formatted as "[key]".
func (t *Translator) T(key string) string {
	if bundle, ok := t.bundles[t.lang]; ok {
		if val, ok := bundle[key]; ok {
			return val
		}
	}
	// Fallback to English
	if bundle, ok := t.bundles["en"]; ok {
		if val, ok := bundle[key]; ok {
			return val
		}
	}
	return fmt.Sprintf("[%s]", key)
}

// LoadBundle adds or merges translations for a language.
func LoadBundle(lang string, pairs map[string]string) {
	if bundles[lang] == nil {
		bundles[lang] = make(map[string]string)
	}
	for k, v := range pairs {
		bundles[lang][k] = v
	}
}

// AvailableLanguages returns a list of languages with loaded bundles.
func AvailableLanguages() []string {
	var langs []string
	for lang := range bundles {
		langs = append(langs, lang)
	}
	return langs
}
