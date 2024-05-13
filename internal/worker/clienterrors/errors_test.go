package clienterrors_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

// ensure clienterrors.Error implements the "error" interface
var _ error = &clienterrors.Error{}

type customErr struct{}

func (ce *customErr) Error() string {
	return "customErr"
}

func TestErrorInterface(t *testing.T) {
	for _, tc := range []struct {
		err         []error
		expectedStr string
	}{
		{[]error{fmt.Errorf("some error")}, "[some error]"},
		{[]error{&customErr{}}, "[customErr]"},
	} {
		wce := clienterrors.WorkerClientError(2, "reason", tc.err)
		assert.Equal(t, fmt.Sprintf("Code: 2, Reason: reason, Details: %s", tc.expectedStr), wce.Error())
	}
}

func TestErrorJSONMarshal(t *testing.T) {
	for _, tc := range []struct {
		err         []error
		expectedStr string
	}{
		{[]error{fmt.Errorf("some-error")}, `["some-error"]`},
		{[]error{fmt.Errorf("err1"), fmt.Errorf("err2")}, `["err1","err2"]`},
	} {
		json, err := json.Marshal(clienterrors.WorkerClientError(2, "reason", tc.err))
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf(`{"id":2,"reason":"reason","details":%s}`, tc.expectedStr), string(json))
	}
}
