package certificate

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/certificate-transparency-go/asn1"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509/pkix"
	"github.com/google/certificate-transparency-go/x509util"
)

// utils.go is adapted from:
// https://github.com/google/certificate-transparency-go/blob/eec07d409a1c1cf99c90cac677b1d8005090d7e8/x509util/x509util.go

// OIDInExtensions checks whether the extension identified by oid is present in extensions
// and returns how many times it occurs together with an indication of whether any of them
// are marked critical.
func OIDInExtensions(oid asn1.ObjectIdentifier, extensions []pkix.Extension) (int, bool) {
	count := 0
	critical := false
	for _, ext := range extensions {
		if ext.Id.Equal(oid) {
			count++
			if ext.Critical {
				critical = true
			}
		}
	}
	return count, critical
}

func getBasicConstraints(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionBasicConstraints, cert.Extensions)
	if count > 0 {
		buf.WriteString(fmt.Sprintf("CA:%t", cert.IsCA))
		if cert.MaxPathLen > 0 || cert.MaxPathLenZero {
			buf.WriteString(fmt.Sprintf(", pathlen:%d", cert.MaxPathLen))
		}
	}
	return buf.String()
}

func getAuthInfoAccess(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionAuthorityInfoAccess, cert.Extensions)
	if count > 0 {
		var issuerBuf bytes.Buffer
		for _, issuer := range cert.IssuingCertificateURL {
			commaAppend(&issuerBuf, "URI:"+issuer)
		}
		if issuerBuf.Len() > 0 {
			buf.WriteString(fmt.Sprintf("CA Issuers - %v", issuerBuf.String()))
		}
		var ocspBuf bytes.Buffer
		for _, ocsp := range cert.OCSPServer {
			commaAppend(&ocspBuf, "URI:"+ocsp)
		}
		if ocspBuf.Len() > 0 {
			buf.WriteString(fmt.Sprintf("OCSP - %v", ocspBuf.String()))
		}
	}
	return buf.String()
}

func getAuthKeyID(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionAuthorityKeyId, cert.Extensions)
	if count > 0 {
		buf.WriteString(fmt.Sprintf("keyid:%v", hex.EncodeToString(cert.AuthorityKeyId)))
	}
	return buf.String()
}

func getCertPolicies(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionCertificatePolicies, cert.Extensions)
	if count > 0 {
		for _, oid := range cert.PolicyIdentifiers {
			buf.WriteString(fmt.Sprintf("Policy: %v", oid.String()))
		}
	}
	return buf.String()
}

func getCRLDPs(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionCRLDistributionPoints, cert.Extensions)
	if count > 0 {
		buf.WriteString("Full Name:")
		var bufPoints bytes.Buffer
		for _, pt := range cert.CRLDistributionPoints {
			commaAppend(&bufPoints, "URI:"+pt)
		}
		buf.WriteString(fmt.Sprintf("%v", bufPoints.String()))
	}
	return buf.String()
}

func getSubjectAltName(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionSubjectAltName, cert.Extensions)
	if count > 0 {
		for _, name := range cert.DNSNames {
			commaAppend(&buf, "DNS:"+name)
		}
		for _, email := range cert.EmailAddresses {
			commaAppend(&buf, "email:"+email)
		}
		for _, ip := range cert.IPAddresses {
			commaAppend(&buf, "IP Address:"+ip.String())
		}
	}
	return buf.String()
}

func getSubjectKeyID(cert *x509.Certificate) string {
	buf := bytes.Buffer{}
	count, _ := OIDInExtensions(x509.OIDExtensionSubjectKeyId, cert.Extensions)
	if count > 0 {
		buf.WriteString(fmt.Sprintf("keyid:%v", hex.EncodeToString(cert.SubjectKeyId)))
	}
	return buf.String()
}
func getDomains(altNames string) []string {
	var domains []string
	pieces := strings.Split(altNames, ",")
	for _, piece := range pieces {
		if strings.Contains(piece, "DNS:") {
			domain := strings.TrimPrefix(strings.TrimPrefix(piece, "DNS:"), " DNS:")
			domains = append(domains, domain)
		}
	}
	return domains
}

func keyUsageToString(k x509.KeyUsage) string {
	var buf bytes.Buffer
	if k&x509.KeyUsageDigitalSignature != 0 {
		commaAppend(&buf, "Digital Signature")
	}
	if k&x509.KeyUsageContentCommitment != 0 {
		commaAppend(&buf, "Content Commitment")
	}
	if k&x509.KeyUsageKeyEncipherment != 0 {
		commaAppend(&buf, "Key Encipherment")
	}
	if k&x509.KeyUsageDataEncipherment != 0 {
		commaAppend(&buf, "Data Encipherment")
	}
	if k&x509.KeyUsageKeyAgreement != 0 {
		commaAppend(&buf, "Key Agreement")
	}
	if k&x509.KeyUsageCertSign != 0 {
		commaAppend(&buf, "Certificate Signing")
	}
	if k&x509.KeyUsageCRLSign != 0 {
		commaAppend(&buf, "CRL Signing")
	}
	if k&x509.KeyUsageEncipherOnly != 0 {
		commaAppend(&buf, "Encipher Only")
	}
	if k&x509.KeyUsageDecipherOnly != 0 {
		commaAppend(&buf, "Decipher Only")
	}
	return buf.String()
}

func commaAppend(buf *bytes.Buffer, s string) {
	if buf.Len() > 0 {
		buf.WriteString(", ")
	}
	buf.WriteString(s)
}

func GetInfo(cert *x509.Certificate) *X509CertificateInfo {
	certInfo := &X509CertificateInfo{
		Subject: X509Subject{
			Aggregated: x509util.NameToString(cert.Subject),
			C:          toFlatString(cert.Subject.Country),
			ST:         toFlatString(cert.Subject.Province),
			L:          toFlatString(cert.Subject.Locality),
			O:          toFlatString(cert.Subject.Organization),
			OU:         toFlatString(cert.Subject.OrganizationalUnit),
			CN:         cert.Subject.CommonName,
		},
		Extensions: X509Extensions{
			BasicConstraints:       getBasicConstraints(cert),
			KeyUsage:               keyUsageToString(cert.KeyUsage),
			AuthorityInfoAccess:    getAuthInfoAccess(cert),
			AuthorityKeyIdentifier: getAuthKeyID(cert),
			CertificatePolicies:    getCertPolicies(cert),
			CRLDistributionPoints:  getCRLDPs(cert),
			SubjectKeyIdentifier:   getSubjectKeyID(cert),
			SubjectAltNames:        getSubjectAltName(cert),
		},
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		AsDER:     base64.StdEncoding.EncodeToString(cert.Raw),
		Domains:   getDomains(getSubjectAltName(cert)),
	}
	return certInfo
}

func toFlatString(input []string) string {
	return strings.Join(input, " ")
}

func GetSourceName(uri string) string {
	pieces := strings.Split(uri, "/")
	return pieces[len(pieces)-1]
}
