package googledrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"google.golang.org/api/googleapi"
	"image"
	"image/png"
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
	path []string

	jsonToken string
	config    *oauth2.Config
	token     *oauth2.Token
	client    *http.Client
	service   *drive.Service

	SetToken           func(string)
	EnterAuthorization func() string
}

// New creates a new Google Drive "Manager", it's the object that manages authentication, authorization
// tokens, configs and communication with GoogleDrive.
//
// All files are create within the fixed given `path` (list of strings).
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
func New(ctx context.Context, path []string, token string, setToken func(token string), enterAuthorization func() string) (*Manager, error) {
	m := &Manager{
		path:               path,
		jsonToken:          token,
		SetToken:           setToken,
		EnterAuthorization: enterAuthorization,
	}
	var err error
	m.config, err = google.ConfigFromJSON([]byte(credentialsJson),
		/* Scope of authorizations: */ drive.DriveFileScope)
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

	m.client, err = m.getClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	m.service, err = drive.NewService(ctx, option.WithHTTPClient(m.client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve GoogleDrive client: %w", err)
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
		_ = json.NewEncoder(&b).Encode(m.token)
		m.jsonToken = b.String()
		if m.SetToken != nil {
			m.SetToken(m.jsonToken)
		}
	}
	return m.config.Client(ctx, m.token), nil
}

// Request a token from the web, then returns the retrieved token.
func (m *Manager) getTokenFromWeb() (*oauth2.Token, error) {
	// Scope of authorization is given in the config object.
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
func (m *Manager) ShareImage(ctx context.Context, name string, img image.Image) (url string, err error) {
	parentId, err := m.createPath(ctx)
	if err != nil {
		return "", err
	}

	// Create PNG content of the image.
	var contentBuffer bytes.Buffer
	_ = png.Encode(&contentBuffer, img)
	content := contentBuffer.Bytes()

	f := &drive.File{
		MimeType: "image/png",
		Name:     name + ".png",
		Parents:  []string{parentId},
	}
	f, err = m.service.Files.Create(f).
		Context(ctx).
		Media(bytes.NewReader(content)).
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	glog.V(2).Infof("Returned file: %+v", f)

	// Make uploaded image visible (but not writeable) to all.
	_, err = m.service.Permissions.Create(f.Id, &drive.Permission{
		AllowFileDiscovery: false,
		Role:               "reader",
		Type:               "anyone",
		View:               "",
		ServerResponse:     googleapi.ServerResponse{},
		ForceSendFields:    nil,
		NullFields:         nil,
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create shared read permissions for file name=%q id=%q: %w",
			f.Name, f.Id, err)
	}

	// Get link to image.
	f2, err := m.service.Files.Get(f.Id).Fields("webViewLink").Do()
	if err != nil {
		return "", fmt.Errorf("failed to get shared link to file name=%q id=%q: %w",
			f.Name, f.Id, err)
	}
	glog.V(2).Infof("- WebLinkView=%s", f2.WebViewLink)
	return f2.WebViewLink, nil
}

const folderMimeType = "application/vnd.google-apps.folder"

// createPath creates the path for the manager, if it doesn't yet exist.
func (m *Manager) createPath(ctx context.Context) (id string, err error) {
	var parents []string
	subPath := m.path

	id = "root"
	for len(subPath) > 0 {
		fileList, err := m.service.Files.List().
			Q(fmt.Sprintf("mimeType = '%s' and trashed=false and '%s' in parents and name='%s'",
				folderMimeType, id, subPath[0])).
			Do()
		if err != nil {
			err = fmt.Errorf("failed to find subdirectory %q in %v: %w", subPath[0], parents, err)
			glog.Errorf("googledrive.Manager.createPath: %v", err)
			return "", err
		}
		if len(fileList.Files) == 0 {
			f := &drive.File{
				MimeType: folderMimeType,
				Name:     subPath[0],
				Parents:  []string{id},
			}
			f, err = m.service.Files.Create(f).
				Context(ctx).
				Do()
			if err != nil {
				glog.Warningf("Failed to create sub-folder %q in %v: %v", subPath[0], parents, err)
			}
			id = f.Id
		} else {
			// Directory found: take the first one, since GoogleDrive allows multiple files/folders with the same name.
			id = fileList.Files[0].Id
		}
		glog.V(2).Infof("Path part %q: id=%q", subPath[0], id)
		parents = append(parents, subPath[0])
		subPath = subPath[1:]
	}
	return
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
