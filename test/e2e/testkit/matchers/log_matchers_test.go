//go:build e2e

package matchers

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConsistOfNumberOfLogs", Label("logging"), func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should match 0", func() {
			success, err := ConsistOfNumberOfLogs(0).Match(nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeTrue())
		})
	})

	Context("with empty input", func() {
		It("should match 0", func() {
			success, err := ConsistOfNumberOfLogs(0).Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeTrue())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := ConsistOfNumberOfLogs(0).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_logs/logs.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(ConsistOfNumberOfLogs(28))
		})
	})

})

var _ = Describe("ContainLogs", Label("logging"), func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainLogs().Match(nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := ContainLogs().Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := ContainLogs().Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_logs/logs.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(ContainLogs())
		})
	})
})

var _ = Describe("ContainsLogsWith", Label("logging"), func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainsLogsWith("mock_namespace", "mock_pod", "mock_container").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := ContainsLogsWith("mock_namespace", "mock_pod", "mock_container").Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := ContainsLogsWith("mock_namespace", "mock_pod", "mock_container").Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_logs/logs.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed with namespace", func() {
			Expect(fileBytes).Should(ContainsLogsWith("log-mocks-single-pipeline", "", ""))
		})

		It("should succeed with pod", func() {
			Expect(fileBytes).Should(ContainsLogsWith("log-mocks-single-pipeline", "log-receiver", ""))
		})

		It("should succeed with container", func() {
			Expect(fileBytes).Should(ContainsLogsWith("log-mocks-single-pipeline", "", "fluentd"))
		})

		It("should fail with namespace", func() {
			Expect(fileBytes).ShouldNot(ContainsLogsWith("not-exist", "", ""))
		})

		It("should fail with pod", func() {
			Expect(fileBytes).ShouldNot(ContainsLogsWith("", "not-exist", ""))
		})
	})
})

var _ = Describe("ContainsLogsKeyValue", Label("logging"), func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should not match", func() {
			success, err := ContainsLogsKeyValue("mockKey", "mockKey").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := ContainsLogsKeyValue("mockKey", "mockKey").Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := ContainsLogsKeyValue("mockKey", "mockKey").Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_logs/logs.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed with key value", func() {
			Expect(fileBytes).Should(ContainsLogsKeyValue("user", "foo"))
		})

		It("should fail with value", func() {
			Expect(fileBytes).ShouldNot(ContainsLogsKeyValue("user", "not-exist"))
		})

		It("should fail with key", func() {
			Expect(fileBytes).ShouldNot(ContainsLogsKeyValue("key-not-exist", "foo"))
		})
	})
})

var _ = Describe("HasKubernetesLabels", Label("logging"), func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should not match", func() {
			success, err := HasKubernetesLabels().Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := HasKubernetesLabels().Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HasKubernetesLabels().Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_logs/logs.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HasKubernetesLabels())
		})
	})
})

var _ = Describe("HasKubernetesAnnotations", Label("logging"), func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should not match", func() {
			success, err := HasKubernetesAnnotations().Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match", func() {
			success, err := HasKubernetesAnnotations().Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HasKubernetesAnnotations().Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having logs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_logs/logs.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HasKubernetesAnnotations())
		})
	})
})
