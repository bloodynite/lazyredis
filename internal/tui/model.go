package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/bloodynite/lazyredis/internal/config"
	"github.com/bloodynite/lazyredis/internal/store"
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
	detailGen    uint64
	// DetailTotal is the O(1) length reported by the Redis summary for
	// the selected composite key (-1 for strings and unknown types).
	// DetailLoaded is how many entries of DetailTotal are currently in
	// m.KeyDetail. The UI silently loads the next chunk when the user
	// scrolls toward the end.
	DetailTotal         int64
	DetailLoaded        int
	detailChunkPending  bool
	// detailRetryCount bounds how many times the same selection will
	// retry after a transient Redis error (most commonly WRONGTYPE
	// caused by the key changing type between the summary pipeline and
	// the detail fetch). Reset to 0 on every selection change.
	detailRetryCount    uint8

	refreshGen uint64

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

	DetailSearchInput textinput.Model
	DetailSearchFocus bool
	// DetailSearchMatches holds the positions of every match produced by the
	// most recent detail search apply. For strings each entry is a byte index
	// of a substring occurrence in the sanitized detail value (newlines
	// collapsed to the visible marker, the same text the renderer chunks);
	// for composite types it is the item index of the matching entry.
	// DetailSearchCursor is the active match index inside this slice (-1
	// when empty).
	DetailSearchMatches []int
	DetailSearchCursor  int

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

	detailSearch := textinput.New()
	detailSearch.Placeholder = "search value"
	detailSearch.CharLimit = 200
	detailSearch.Width = 40

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
		Screen:            ScreenProfiles,
		Spinner:           s,
		SearchInput:       search,
		DetailSearchInput: detailSearch,
		EditInput:         edit,
		FormInputs:        inputs,
		NewKeyTTL:         newKeyTTL,
		NewKeyName:        newKeyName,
		NewKeyValue:       newKeyValue,
		ScanPattern:       "*",
		refreshGen:        1,
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
