package smsc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

const DefaultURL = "https://smsc.ru/sys/send.php"

var ErrNoLoginPassword = errors.New("smsc: empty login or password")

// Config is a Client config.
type Config struct {
	URL      string
	Login    string
	Password string
	Client   *http.Client

	// TODO: Add flag to send md5 hash of password in requests.
	// TODO: Add Opt to be applied to all messages by default. The option must
	// be overriden by options passed into Send().
}

// New initializes a Client.
func New(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		cfg.URL = DefaultURL
	}
	if cfg.Login == "" || cfg.Password == "" {
		return nil, ErrNoLoginPassword
	}
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	c := &Client{
		url:      cfg.URL,
		login:    cfg.Login,
		password: cfg.Password,
		http:     cfg.Client,
	}
	return c, nil
}

// Client has methods for API calls.
type Client struct {
	url      string
	login    string
	password string
	http     *http.Client
}

func (c *Client) Send(text string, phones []string, opts ...Opt) (*Result, error) {
	m := &message{
		Login:    c.login,
		Password: c.password,
		Text:     text,
		Phones:   phones,
		Format:   formatJSON,
	}
	for _, opt := range opts {
		opt(m)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	resp, err := c.http.PostForm(c.url, m.Values())
	if err != nil {
		return nil, wrapErr(err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapErr(err)
	}

	// MetaResult allows to parse Result and Error from response while
	// structures have different fields.
	type MetaResult struct {
		*Result
		*Error
	}

	var mr *MetaResult
	if err := json.Unmarshal(b, &mr); err != nil {
		return nil, wrapErr(err)
	}
	if mr.Error != nil {
		return nil, mr.Error
	}
	return mr.Result, nil
}

// wrapErr add smsc package prefix to error.
func wrapErr(err error) error {
	if err == nil {
		return err
	}
	return fmt.Errorf("smsc: %s", err)
}

// message controls what and how will sent to API.
type message struct {
	Login    string
	Password string
	Text     string
	Phones   []string
	Format   format
	Cost     Cost
}

const (
	textMaxLen           = 800
	smsMaxLen            = 160
	smsMaxLenCyrillic    = 70
	smsHeaderLen         = 7
	smsHeaderLenCyrillic = 3
)

// Validate checks the message integrity and returns an optional error.
func (m *message) Validate() error {
	// TODO: Validate length of the text.
	// TODO: Validate phone numbers exist.
	// TODO: Validate other options?
	return nil
}

// Values returns a form for a request to API.
func (m *message) Values() url.Values {
	v := url.Values{
		"login":  []string{m.Login},
		"psw":    []string{m.Password},
		"mes":    []string{m.Text},
		"phones": m.Phones,
	}
	if m.Format != 0 {
		v.Set("fmt", strconv.FormatInt(int64(m.Format), 10))
	}
	if m.Cost != 0 {
		v.Set("cost", strconv.FormatInt(int64(m.Cost), 10))
	}
	return v
}

// Opt configures a send message and a Result.
type Opt func(*message)

// TODO: Add generic With. Such multi Opt could be used in Config for Client.
//
//     func With(opts ...Opt) Opt
//

func WithCost(c Cost) Opt {
	return func(m *message) {
		m.Cost = c
	}
}

// format defines API output format.
type format int

const (
	formatInlineVerbose = 0
	formatInline        = 1
	formatXML           = 2
	formatJSON          = 3
)

// Cost defines whether API should send a Result with cost information.
type Cost int

const (
	CostOmit         Cost = 0
	CostWithoutSend       = 1
	CostCount             = 2
	CostCountBalance      = 3
)

// TODO: Add more options.
// Major - Op, ID, Sender, Translit, Subj, Charset.
// Minor - TinyURL, Time, Tz, Period, Freq, Flash, Bin, Push, HLR, Ping, MMS,
// Mail, Viber, FileURL, Call, Voice, List, Valid, MaxSMS, ImgCode, UserIP, Err,
// PP.

type Result struct {
	ID      int     `json:"id"`
	Count   int     `json:"count"`
	Cost    *string `json:"cost"`
	Balance *string `json:"balance"`
}

func (r *Result) String() string {
	return fmt.Sprintf("OK - %d SMS, ID - %d", r.Count, r.ID)
}

type Error struct {
	Code int    `json:"error_code"`
	Desc string `json:"error"`
	ID   *int   `json:"id"`
}

func (e *Error) Error() string {
	s := fmt.Sprintf("ERROR = %d (%s)", e.Code, e.Desc)
	if e.ID != nil {
		s += fmt.Sprintf(", ID - %d", *e.ID)
	}
	return s
}
