package stream

import (
	"io"
	"net/http"

	"github.com/google/certificate-transparency-go/loglist3"
	"github.com/thediveo/enumflag/v2"
)

type LogStatus enumflag.Flag

const (
	Usable LogStatus = iota
	Undefined
	Retired
	Rejected
	ReadOnly
	Pending
	Qualified
)

var (
	// cache response from the "all log list" URL
	logList *loglist3.LogList

	LogStatusIds = map[LogStatus][]string{
		Usable:    {"usable"},
		Undefined: {"undefined"},
		Retired:   {"retired"},
		ReadOnly:  {"readonly"},
		Rejected:  {"rejected"},
		Pending:   {"pending"},
		Qualified: {"qualified"},
	}

	// Do not use the following Log Types ...
	// >Rejected. When all certificates contained in a CT Log have expired and the CT Log
	// is no longer issuing new SCTs in response to logging requests, it will transition
	// into the Rejected state.
	// >Retired. A Retired Log is one that was at one point Qualified , but has stopped
	// being relied upon for the creation of new SCTs. CT Logs usually enter the Retired
	// state due to a failure to adhere to the ongoing requirements outlined in the
	// Certificate Transparency Log Policy.
	logStatusToLLStatusMap = map[LogStatus]loglist3.LogStatus{
		Usable:    loglist3.UsableLogStatus,
		Undefined: loglist3.UndefinedLogStatus,
		Retired:   loglist3.RetiredLogStatus,
		ReadOnly:  loglist3.ReadOnlyLogStatus,
		Rejected:  loglist3.RejectedLogStatus,
		Pending:   loglist3.PendingLogStatus,
		Qualified: loglist3.QualifiedLogStatus,
	}
)

func GetLogList() (*loglist3.LogList, error) {
	if logList != nil {
		return logList, nil
	}
	httpClient := &http.Client{}
	resp, err := httpClient.Get(loglist3.AllLogListURL)
	if err != nil {
		return nil, err
	}
	json, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ll, err := loglist3.NewFromJSON(json)
	if err != nil {
		return nil, err
	}
	logList = ll
	return ll, nil
}
