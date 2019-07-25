package web

import (
	"strings"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"golang.org/x/xerrors"
)

var (
	// ErrNoAuth is an error returned by the 'NewTwitter' function when the supplied
	// credentials are nil.
	ErrNoAuth = xerrors.New("twitter credentials cannot be nil")
	// ErrEmptyFilter is an error returned by the 'NewTwitter' function when the supplied
	// keyword filter list is empty.
	ErrEmptyFilter = xerrors.New("twitter stream filter cannot be empty or nil")
	// ErrAlreadyStarted is an error returned by the 'Filter' function when the filter is currently
	// running and an attempt to start it again was made.
	ErrAlreadyStarted = xerrors.New("twitter stream already started")
)

// Tweet is a simple struct to abstract out non-important Tweet data.
type Tweet struct {
	ID        int64
	User      string
	Text      string
	Time      int64
	Images    []string
	UserName  string
	UserPhoto string
}

// Filter is a struct that allows for filtering Tweets via Test
// or Sender.
type Filter struct {
	Language     []string `json:"language"`
	Keywords     []string `json:"keywords"`
	OnlyUsers    []string `json:"only_users"`
	BlockedUsers []string `json:"blocked_users"`
	BlockedWords []string `json:"banned_words"`
}

// Twitter is a struct to hold and operate with the Twitter client, including
// using timeouts.
type Twitter struct {
	cb     func(*Tweet)
	filter *Filter
	stream *twitter.Stream
	client *twitter.Client
}

// Credentials is a struct used to store and access the Twitter API keys.
type Credentials struct {
	AccessKey      string `json:"access_key"`
	ConsumerKey    string `json:"consomer_key"`
	AccessSecret   string `json:"access_secret"`
	ConsumerSecret string `json:"consomer_secret"`
}

// Stop will stop the filter process, if running.
func (t *Twitter) Stop() {
	if t.stream != nil {
		t.stream.Stop()
	}
}

// Start kicks off the Twitter stream filter and receiver. This function DOES NOT block and returns an
// error of nil if successful.
func (t *Twitter) Start() error {
	if t.stream != nil {
		return ErrAlreadyStarted
	}
	s, err := t.client.Streams.Filter(&twitter.StreamFilterParams{
		Track:         t.filter.Keywords,
		Language:      t.filter.Language,
		StallWarnings: twitter.Bool(true),
	})
	if err != nil {
		return xerrors.Errorf("unable to start Twitter filter: %w", err)
	}
	t.stream = s
	d := twitter.NewSwitchDemux()
	d.Tweet = t.receive
	go func(x *Twitter, q twitter.SwitchDemux) {
		for m := range x.stream.Messages {
			q.Handle(m)
		}
		x.stream = nil
	}(t, d)
	return nil
}
func (f *Filter) match(u, c string) bool {
	if len(f.BlockedUsers) > 0 {
		for i := range f.BlockedUsers {
			if strings.ToLower(f.BlockedUsers[i]) == u {
				return false
			}
		}
	}
	if len(f.BlockedWords) > 0 {
		for i := range f.BlockedWords {
			if strings.Contains(c, f.BlockedWords[i]) {
				return false
			}
		}
	}
	if len(f.OnlyUsers) > 0 {
		for i := range f.OnlyUsers {
			if strings.ToLower(f.OnlyUsers[i]) == u {
				return true
			}
		}
		return false
	}
	return true
}

// Callback sets the function to be called when a Tweet matching the Filter is received.
func (t *Twitter) Callback(f func(*Tweet)) {
	t.cb = f
}
func (t *Twitter) receive(x *twitter.Tweet) {
	if t.filter != nil {
		if !t.filter.match(strings.ToLower(x.User.ScreenName), x.Text) {
			return
		}
	}
	r := &Tweet{
		ID:        x.ID,
		User:      x.User.ScreenName,
		Text:      x.Text,
		UserName:  x.User.Name,
		UserPhoto: x.User.ProfileImageURLHttps,
	}
	if len(x.Entities.Media) > 0 {
		r.Images = make([]string, 0, len(x.Entities.Media))
		for i := range x.Entities.Media {
			if x.Entities.Media[i].Type == "photo" {
				r.Images = append(r.Images, x.Entities.Media[i].MediaURLHttps)
			}
		}
	}
	if t.cb != nil {
		t.cb(r)
	}
}

// NewTwitter creates and establishes a Twitter session with the provided Access and Consumer Keys/Secrets
// and a Timeout. This function will return an error if it cannot reach Twitter or authentication failed.
func NewTwitter(timeout time.Duration, f *Filter, a *Credentials) (*Twitter, error) {
	if a == nil {
		return nil, ErrNoAuth
	}
	if f == nil || len(f.Keywords) == 0 {
		return nil, ErrEmptyFilter
	}
	c := oauth1.NewConfig(a.ConsumerKey, a.ConsumerSecret)
	i := c.Client(oauth1.NoContext, oauth1.NewToken(a.AccessKey, a.AccessSecret))
	i.Timeout = timeout
	t := &Twitter{
		filter: f,
		client: twitter.NewClient(i),
	}
	if _, _, err := t.client.Accounts.VerifyCredentials(nil); err != nil {
		return nil, xerrors.Errorf("cannot authenticate to Twitter: %w", err)
	}
	return t, nil
}