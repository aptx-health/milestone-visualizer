package gh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

// NewClient returns an authenticated GitHub client.
// Auth precedence: GITHUB_TOKEN env, then `gh auth token`.
func NewClient(ctx context.Context) (*github.Client, error) {
	tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if tok == "" {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return nil, fmt.Errorf("no GITHUB_TOKEN and `gh auth token` failed: %w", err)
		}
		tok = strings.TrimSpace(string(out))
	}
	if tok == "" {
		return nil, fmt.Errorf("could not resolve GitHub token")
	}
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
	return github.NewClient(oauth2.NewClient(ctx, src)), nil
}

// ParseOwnerRepo splits "owner/repo" into its parts.
func ParseOwnerRepo(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected <owner>/<repo>, got %q", s)
	}
	return parts[0], parts[1], nil
}
