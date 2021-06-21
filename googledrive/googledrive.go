package googledrive

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"image"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	credentialsJson = `{"installed":{"client_id":"619679595672-si1u5f4jhqpjgubkqtaart5hnfdqr0s6.apps.googleusercontent.com","project_id":"goshot","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs","client_secret":"XgBNHOn-zC3W9ky2YDXkGr_o","redirect_uris":["urn:ietf:wg:oauth:2.0:oob","http://localhost"]}}`
)

// Manager is the object that manages Google Drive's credentials, authentication,
// config, token, etc.
type Manager struct {
	jsonToken string
	token     *oauth2.Token
	config    *oauth2.Config

	SetToken           func(string)
	EnterAuthorization func() string
}

// New creates a new Google Drive "Manager", it's the object that manages authentication, authorization
// tokens, configs and communication with GoogleDrive.
//
// A previously saved authorization `token` can be passed to reuse authorization. If none are available simply pass
// an empty string here, and a new authorization will be requested.
//
// It requires the following callbacks to be defined:
//
// * setToken: if the token that gives temporary permission to access Google Drive is updated, this
//   function is called. This token can be saved re-used later on, by passing it in the `token` parameter.
//   If nil, it won't be called, and the token will be "forgotten" at the next instance of the program.
// * enterAuthorization: when a new token is required, the Manager will open the browser with Google
//   to ask for a renewed authentication. Then it will call this function to have a UI for the user
//   to paste the authorization string given by Google.
//
// May return an error if application credentials are wrong.
func New(token string, setToken func(token string), enterAuthorization func() string) (*Manager, error) {
	m := &Manager{
		jsonToken:          token,
		SetToken:           setToken,
		EnterAuthorization: enterAuthorization,
	}
	var err error
	m.config, err = google.ConfigFromJSON([]byte(credentialsJson), drive.DriveMetadataReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}

	if m.jsonToken != "" {
		tok := &oauth2.Token{}
		err = json.NewDecoder(strings.NewReader(m.jsonToken)).Decode(tok)
		if err != nil {
			glog.Errorf("unable to parse token from what was previously saved, ignoring it: %s", err)
			m.jsonToken = ""
			m.token = nil
		} else {
			m.token = tok
		}
	}

	return m, nil
}

// Retrieve a token, saves the token, then returns the generated client.
func (m *Manager) getClient(ctx context.Context) (*http.Client, error) {
	var err error
	if m.jsonToken == "" || m.token == nil {
		// Acquire new authorization token -- that means open the authorization url in the browser,
		// and opening some form of UI (a dialog box?) for the user to paste the authorization.
		m.token, err = m.getTokenFromWeb()
		if err != nil {
			return nil, fmt.Errorf("failed to get authorization from the web: %w", err)
		}
		var b strings.Builder
		json.NewEncoder(&b).Encode(m.token)
		m.jsonToken = b.String()
		if m.SetToken != nil {
			m.SetToken(m.jsonToken)
		}
	}
	return m.config.Client(context.Background(), m.token), nil
}

// Request a token from the web, then returns the retrieved token.
func (m *Manager) getTokenFromWeb() (*oauth2.Token, error) {
	authURL := m.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	if err := openurl(authURL); err != nil {
		return nil, err
	}
	authCode := m.EnterAuthorization()
	if authCode == "" {
		return nil, fmt.Errorf("no GoogleDrive authorization given")
	}

	tok, err := m.config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web with authorization given: %w", err)
	}
	return tok, nil
}

// ShareImage will create
func (m *Manager) ShareImage(_ image.Image) (url string, err error) {
	ctx := context.Background()

	// If modifying these scopes, delete your previously saved token.json.
	client, err := m.getClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP client: %w", err)
	}

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("unable to retrieve GoogleDrive client: %w", err)
	}

	r, err := srv.Files.List().PageSize(10).
		Fields("nextPageToken, files(id, name)").Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve GoogleDrive files: %w", err)
	}
	fmt.Println("Files:")
	if len(r.Files) == 0 {
		fmt.Println("No files found.")
	} else {
		for _, i := range r.Files {
			fmt.Printf("%s (%s)\n", i.Name, i.Id)
		}
	}
	return "Not implemented yet!", nil
}

func openurl(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	}
	return fmt.Errorf("unsupported platform %q -- don't know how to open a browser", runtime.GOOS)
}
