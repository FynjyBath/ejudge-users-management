package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const changeRegistrationPath = "/ej/api/v1/master/change-registration"

type actionType string

const (
	actionRegister   actionType = "register"
	actionUnregister actionType = "unregister"
)

type userSpec struct {
	ID    *int
	Login string
	Name  string
}

type changeRegistrationReply struct {
	OK     bool        `json:"ok"`
	Result bool        `json:"result"`
	Error  *replyError `json:"error"`
	Action string      `json:"action"`
}

type replyError struct {
	Message string `json:"message"`
	Num     int    `json:"num"`
	Symbol  string `json:"symbol"`
	LogID   string `json:"log_id"`
}

type config struct {
	Token string `json:"token"`
}

func main() {
	var (
		usersArg    = flag.String("users", "", "List of users in the format id:name;login2:name2")
		contestsArg = flag.String("contests", "", "Semicolon separated list of contest IDs")
		actionArg   = flag.String("action", string(actionRegister), "Action to perform: register or unregister")
		baseURLArg  = flag.String("base-url", "http://localhost", "Base URL of the ejudge installation")
		tokenArg    = flag.String("token", "", "Value for the Authorization header; overrides the value from the config file")
		configArg   = flag.String("config", "", "Path to JSON configuration file with secrets (e.g. token)")
		timeoutArg  = flag.Duration("timeout", 15*time.Second, "Timeout for each HTTP request")
		insecureArg = flag.Bool("insecure", false, "Skip TLS certificate verification")
	)
	flag.Parse()

	if *usersArg == "" {
		log.Fatal("the -users flag is required")
	}
	if *contestsArg == "" {
		log.Fatal("the -contests flag is required")
	}

	action, err := parseAction(*actionArg)
	if err != nil {
		log.Fatalf("invalid action: %v", err)
	}

	users, err := parseUsers(*usersArg)
	if err != nil {
		log.Fatalf("failed to parse users: %v", err)
	}

	contests, err := parseContestIDs(*contestsArg)
	if err != nil {
		log.Fatalf("failed to parse contests: %v", err)
	}

	cfg, err := loadConfig(strings.TrimSpace(*configArg))
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	token := strings.TrimSpace(*tokenArg)
	if token == "" {
		token = strings.TrimSpace(cfg.Token)
	}
	if token == "" {
		log.Fatal("no API token provided: specify it via -token flag or in the config file")
	}

	baseURL := strings.TrimSuffix(*baseURLArg, "/")
	if baseURL == "" {
		log.Fatal("the base URL must not be empty")
	}

	httpClient := &http.Client{Timeout: *timeoutArg}
	if *insecureArg {
		if baseTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			transport := baseTransport.Clone()
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			httpClient.Transport = transport
		} else {
			log.Fatal("http.DefaultTransport is not *http.Transport; cannot enable -insecure option")
		}
	}

	var failures []string
	for _, contestID := range contests {
		for _, user := range users {
			if err := changeRegistration(httpClient, baseURL, token, contestID, user, action, *timeoutArg); err != nil {
				failures = append(failures, fmt.Sprintf("contest %d, user %s: %v", contestID, userIdentifier(user), err))
			} else {
				log.Printf("%s user %s for contest %d", actionVerb(action), userIdentifier(user), contestID)
			}
		}
	}

	if len(failures) > 0 {
		for _, failure := range failures {
			log.Println("error:", failure)
		}
		os.Exit(1)
	}
}

func parseAction(raw string) (actionType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(actionRegister):
		return actionRegister, nil
	case string(actionUnregister):
		return actionUnregister, nil
	default:
		return "", fmt.Errorf("unsupported action %q", raw)
	}
}

func parseUsers(raw string) ([]userSpec, error) {
	items := splitList(raw)
	if len(items) == 0 {
		return nil, errors.New("user list is empty")
	}

	users := make([]userSpec, 0, len(items))
	for _, item := range items {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid user specification %q", item)
		}

		idPart := strings.TrimSpace(parts[0])
		namePart := strings.TrimSpace(parts[1])
		if idPart == "" {
			return nil, fmt.Errorf("user identifier is empty in %q", item)
		}
		if namePart == "" {
			return nil, fmt.Errorf("user name is empty in %q", item)
		}

		user := userSpec{Name: namePart, Login: idPart}
		if id, err := strconv.Atoi(idPart); err == nil {
			user.ID = &id
		}
		users = append(users, user)
	}
	return users, nil
}

func parseContestIDs(raw string) ([]int, error) {
	items := splitList(raw)
	if len(items) == 0 {
		return nil, errors.New("contest list is empty")
	}

	ids := make([]int, 0, len(items))
	for _, item := range items {
		id, err := strconv.Atoi(item)
		if err != nil {
			return nil, fmt.Errorf("invalid contest ID %q: %w", item, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == ','
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func changeRegistration(client *http.Client, baseURL, token string, contestID int, user userSpec, action actionType, timeout time.Duration) error {
	form := url.Values{}
	if user.ID != nil {
		form.Set("other_user_id", strconv.Itoa(*user.ID))
	}
	if user.Login != "" {
		form.Set("other_user_login", user.Login)
	}
	form.Set("contest_id", strconv.Itoa(contestID))

	switch action {
	case actionRegister:
		form.Set("op", "upsert")
		form.Set("status", "ok")
		form.Set("name", user.Name)
		form.Set("ignore", "true")
	case actionUnregister:
		form.Set("op", "delete")
		form.Set("ignore", "true")
	default:
		return fmt.Errorf("unsupported action %q", action)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	endpoint := baseURL + changeRegistrationPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s: %s", resp.Status, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !isJSONContentType(contentType) {
		const maxPreview = 200
		preview := string(body)
		if len(preview) > maxPreview {
			preview = preview[:maxPreview] + "..."
		}
		return fmt.Errorf("unexpected response content type %q (status %s): %s", contentType, resp.Status, preview)
	}

	var reply changeRegistrationReply
	if err := json.Unmarshal(body, &reply); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if !reply.OK || !reply.Result {
		if reply.Error != nil {
			return fmt.Errorf("server error: %s (code %d, symbol %s, log %s)", reply.Error.Message, reply.Error.Num, reply.Error.Symbol, reply.Error.LogID)
		}
		return errors.New("registration change was not acknowledged")
	}

	return nil
}

func userIdentifier(u userSpec) string {
	switch {
	case u.Login != "" && u.ID != nil:
		return fmt.Sprintf("login=%s (id=%d)", u.Login, *u.ID)
	case u.ID != nil:
		return fmt.Sprintf("id=%d", *u.ID)
	case u.Login != "":
		return fmt.Sprintf("login=%s", u.Login)
	default:
		return "<unknown>"
	}
}

func actionVerb(action actionType) string {
	switch action {
	case actionRegister:
		return "Registered"
	case actionUnregister:
		return "Unregistered"
	default:
		return string(action)
	}
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	switch mediaType {
	case "application/json", "text/json", "application/problem+json":
		return true
	default:
		return false
	}
}

func loadConfig(path string) (config, error) {
	if path == "" {
		return config{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return config{}, fmt.Errorf("opening config file %q: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var cfg config
	if err := decoder.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return config{}, errors.New("config file is empty")
		}
		return config{}, fmt.Errorf("decoding config %q: %w", path, err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return config{}, fmt.Errorf("config file %q contains multiple JSON values", path)
		}
		return config{}, fmt.Errorf("decoding config %q: %w", path, err)
	}

	return cfg, nil
}
