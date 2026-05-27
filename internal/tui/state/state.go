// Package state holds the cross-screen Bubble Tea model fields. Keeping these
// in one struct makes it cheap to pass around without ballooning argument
// lists, and gives a single place to add new global state.
package state

// AWSStatus mirrors the kubekit awssession statuses.
type AWSStatus string

const (
	AWSUnknown  AWSStatus = "unknown"
	AWSOK       AWSStatus = "ok"
	AWSExpired  AWSStatus = "expired"
	AWSNoAWS    AWSStatus = "no-aws"
	AWSMismatch AWSStatus = "mismatch"
)

// AppState is the shared root state.
type AppState struct {
	Width  int
	Height int

	KubeContext   string
	KubeNamespace string

	AWSProfile     string
	AWSAccount     string
	AWSCtxAccount  string
	AWSStatus      AWSStatus
	AWSError       string

	Breadcrumbs []string
	KeyHints    []KeyHint

	// SuggestedAWSProfile is set when the active AWS profile and the
	// kube context's account differ AND ~/.aws/config has a profile
	// whose sso_account_id matches the cluster account. The header
	// renders a one-line callout offering the switch on `a`.
	SuggestedAWSProfile string
}

// KeyHint is a per-screen key/action pair shown in the header.
type KeyHint struct {
	Key    string
	Action string
}
