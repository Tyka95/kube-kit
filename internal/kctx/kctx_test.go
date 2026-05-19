package kctx

import "testing"

func TestParseEKSARN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		region  string
		account string
		cluster string
		ok      bool
	}{
		{
			name:    "valid EKS ARN",
			input:   "arn:aws:eks:us-east-1:123456789012:cluster/my-cluster",
			region:  "us-east-1",
			account: "123456789012",
			cluster: "my-cluster",
			ok:      true,
		},
		{
			name:    "valid EKS ARN with hyphenated cluster name",
			input:   "arn:aws:eks:eu-west-1:000000000001:cluster/prod-eks-v2",
			region:  "eu-west-1",
			account: "000000000001",
			cluster: "prod-eks-v2",
			ok:      true,
		},
		{
			name:    "non-EKS context (docker-desktop)",
			input:   "docker-desktop",
			region:  "",
			account: "",
			cluster: "",
			ok:      false,
		},
		{
			name:    "non-EKS context (kind)",
			input:   "kind-my-cluster",
			region:  "",
			account: "",
			cluster: "",
			ok:      false,
		},
		{
			name:    "malformed ARN – missing cluster prefix",
			input:   "arn:aws:eks:us-east-1:123456789012:my-cluster",
			region:  "",
			account: "",
			cluster: "",
			ok:      false,
		},
		{
			name:    "malformed ARN – account too short",
			input:   "arn:aws:eks:us-east-1:12345:cluster/my-cluster",
			region:  "",
			account: "",
			cluster: "",
			ok:      false,
		},
		{
			name:    "empty string",
			input:   "",
			region:  "",
			account: "",
			cluster: "",
			ok:      false,
		},
		{
			name:    "wrong AWS service",
			input:   "arn:aws:iam:us-east-1:123456789012:role/my-role",
			region:  "",
			account: "",
			cluster: "",
			ok:      false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			region, account, cluster, ok := ParseEKSARN(tc.input)
			if ok != tc.ok {
				t.Errorf("ok = %v, want %v", ok, tc.ok)
			}
			if region != tc.region {
				t.Errorf("region = %q, want %q", region, tc.region)
			}
			if account != tc.account {
				t.Errorf("account = %q, want %q", account, tc.account)
			}
			if cluster != tc.cluster {
				t.Errorf("cluster = %q, want %q", cluster, tc.cluster)
			}
		})
	}
}
