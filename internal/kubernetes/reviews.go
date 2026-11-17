package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The review kinds: questions to the API server, made by creating one and
// read from its answer; none is stored, so none can be listed.
//
//	SelfSubjectReview          who am I?              kubectl auth whoami
//	SelfSubjectAccessReview    may I do this?         kubectl auth can-i
//	SelfSubjectRulesReview     what may I do here?    kubectl auth can-i --list
//	SubjectAccessReview        may they do this?      kubectl auth can-i --as
//	LocalSubjectAccessReview   may they, here?
//	TokenReview                whose token is this?

// WhoAmI asks who the client is, like `kubectl auth whoami`: the user
// name, its groups and extra, as the API server sees them.
func (c *Client) WhoAmI(ctx context.Context) (user string, groups []string, err error) {
	review, err := c.clientset.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}
	info := review.Status.UserInfo
	groups = append([]string{}, info.Groups...)
	sort.Strings(groups)
	return info.Username, groups, nil
}

// userText is a user and its groups, one per line.
func userText(user string, groups []string) string {
	return fmt.Sprintf("Username: %s\nGroups:\n  %s", user, strings.Join(groups, "\n  "))
}

// WhoAmIText is WhoAmI as a page of text.
func (c *Client) WhoAmIText(ctx context.Context) (string, error) {
	user, groups, err := c.WhoAmI(ctx)
	return userText(user, groups), err
}
