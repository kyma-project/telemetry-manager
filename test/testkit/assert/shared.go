package assert

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func BackendDataEventuallyMatches1(offset int, t *testing.T, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	t.Helper()
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseEventuallyMatches1(offset+1, t, queryURL, httpBodyMatcher)
}

func BackendDataEventuallyMatches(ctx context.Context, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseEventuallyMatches(ctx, queryURL, httpBodyMatcher)
}

func BackendDataConsistentlyMatches(ctx context.Context, backend *kitbackend.Backend, httpBodyMatcher types.GomegaMatcher) {
	queryURL := suite.ProxyClient.ProxyURLForService(backend.Namespace(), backend.Name(), kitbackend.QueryPath, kitbackend.QueryPort)
	HTTPResponseConsistentlyMatches(ctx, queryURL, httpBodyMatcher)
}

func HTTPResponseEventuallyMatches1(offset int, t *testing.T, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	t.Helper()
	//t.Log("Waiting for the backend to be ready...")

	//t.Logf("The offset is: %d", getOffset())
	EventuallyWithOffset(getOffset(), func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(t.Context(), queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		//return testme(g, resp, httpBodyMatcher)
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, 2, periodic.TelemetryInterval).Should(Succeed(), "")
}

//func testme(g Gomega, resp *http.Response, httpBodyMatcher types.GomegaMatcher) bool {
//	//return g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
//	return false
//}

func HTTPResponseEventuallyMatches(ctx context.Context, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	Eventually(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(ctx, queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func HTTPResponseConsistentlyMatches(ctx context.Context, queryURL string, httpBodyMatcher types.GomegaMatcher) {
	Consistently(func(g Gomega) {
		resp, err := suite.ProxyClient.GetWithContext(ctx, queryURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(httpBodyMatcher))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func getOffset() int {
	pc := make([]uintptr, 32)
	n := runtime.Callers(0, pc) // Capture full stack
	frames := runtime.CallersFrames(pc[:n])

	var startCounting bool
	count := 0

	for {
		frame, more := frames.Next()
		funcName := getFuncName(frame.Function)

		if !startCounting {
			if funcName == "getOffset" {
				startCounting = true
				continue
			}
			continue
		}

		if strings.HasPrefix(funcName, "Test") {
			break
		}

		count++
		if !more {
			break
		}
	}

	return count

}
func getFuncName(fullName string) string {
	if idx := strings.LastIndex(fullName, "."); idx != -1 {
		return fullName[idx+1:]
	}
	return fullName
}
