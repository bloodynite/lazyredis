package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/frankz/lazyredis/internal/config"
	"github.com/frankz/lazyredis/internal/store"
)

type Screen int

const (
	ScreenProfiles Screen = iota
	ScreenProfileForm
	ScreenBrowser
	ScreenKeyEdit
	ScreenConfirm
)

type panelFocus int

const (
	panelKeys panelFocus = iota
	panelDetail
)

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmDeleteKey
	confirmDeleteProfile
	confirmFlushDB
)

type editMode int

const (
	editString editMode = iota
	editElement
	editElementAdd
	editTTL
	editNewKey
	editExistingKey
	editRefreshInterval
)

const (
	newKeyFieldTTL = iota
	newKeyFieldType
	newKeyFieldKey
	newKeyFieldValue
)

var keyFormTypes = []string{"string", "hash", "list", "set", "zset", "stream"}

type Model struct {
	Width  int
	Height int

	Screen     Screen
	PrevScreen Screen

	Config *config.File
	Client *store.Client

	Profiles     []config.Profile
	ProfileCursor int

	FormInputs   []textinput.Model
	FormFocus    int
	FormEditing  bool
	FormOriginal string

	Info        *store.ServerInfo
	Keys        []string
	ScanCursor  uint64
	ScanPattern string
	scanGen     uint64
	KeyCursor   int
	KeyScroll   int

	SelectedKey string
	KeyDetail   *store.KeyDetail
	DetailCursor int
	DetailScroll int

	EditMode     editMode
	EditInput    textinput.Model
	EditField    string
	EditNewType  string

	NewKeyTTL    textinput.Model
	NewKeyName   textinput.Model
	NewKeyValue  textarea.Model
	NewKeyFocus  int
	KeyFormType  string
	NewKeyTypeCursor int

	ConfirmAction confirmAction
	ConfirmTarget string

	Spinner spinner.Model
	Loading bool
	ErrMsg  string
	Status  string
	statusClearGen uint64

	SearchInput textinput.Model
	SearchFocus bool

	PanelFocus panelFocus

	HelpOpen bool
}

func New() *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	search := textinput.New()
	search.Placeholder = "text or pattern (e.g. demo, user:*)"
	search.CharLimit = 200
	search.Width = 40

	edit := textinput.New()
	edit.CharLimit = 10000
	edit.Width = 60

	inputs := newProfileFormInputs()

	newKeyTTL := newFormInput("3600s, 1h or persist")
	newKeyName := newFormInput("my:key")
	newKeyValue := textarea.New()
	newKeyValue.Placeholder = "value"
	newKeyValue.CharLimit = 10000
	configureNewKeyTextarea(&newKeyValue)
	newKeyValue.SetWidth(40)
	newKeyValue.SetHeight(6)

	return &Model{
		Screen:      ScreenProfiles,
		Spinner:     s,
		SearchInput: search,
		EditInput:   edit,
		FormInputs:  inputs,
		NewKeyTTL:   newKeyTTL,
		NewKeyName:  newKeyName,
		NewKeyValue: newKeyValue,
		ScanPattern: "*",
	}
}

func newFormInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 40
	return ti
}

func (m *Model) Init() tea.Cmd {
	initStyles()
	return tea.Batch(
		m.Spinner.Tick,
		loadProfiles(),
	)
}
