package certificate

import "time"

type Batch struct {
	OperatorName  string
	LogSourceName string
	Start         int64
	End           int64
	Logs          []Log
}

type Log struct {
	EntryType string       `json:"entry_type"`
	Body      X509LogEntry `json:"body"`
}

type X509LogEntry struct {
	Cert     X509CertificateInfo   `json:"cert_leaf"`
	Chain    []X509CertificateInfo `json:"cert_chain"`
	Index    int64                 `json:"index"`
	Date     string                `json:"date"`
	IssuedAt time.Time             `json:"issued_at"`
	Source   LogSource             `json:"source"`
}

type X509CertificateInfo struct {
	Subject    X509Subject    `json:"subject"`
	Extensions X509Extensions `json:"extensions"`
	NotBefore  time.Time      `json:"not_before"`
	NotAfter   time.Time      `json:"not_after"`
	AsDER      string         `json:"as_der"`
	Domains    []string       `json:"domains,omitempty"`
}

type X509Subject struct {
	Aggregated string `json:"aggregated"`
	C          string `json:"C"`
	ST         string `json:"ST"`
	L          string `json:"L"`
	O          string `json:"O"`
	OU         string `json:"OU"`
	CN         string `json:"CN"`
}

type X509Extensions struct {
	BasicConstraints       string `json:"basicConstraints,omitempty"`
	KeyUsage               string `json:"keyUsage,omitempty"`
	AuthorityInfoAccess    string `json:"authorityInfoAccess,omitempty"`
	AuthorityKeyIdentifier string `json:"authorityKeyIdentifier,omitempty"`
	CertificatePolicies    string `json:"certificatePolicies,omitempty"`
	CRLDistributionPoints  string `json:"crlDistributionPoints,omitempty"`
	SubjectKeyIdentifier   string `json:"subjectKeyIdentifier,omitempty"`
	SubjectAltNames        string `json:"subjectAltNames,omitempty"`
}

type LogSource struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}
