//go:build e2e

package matchers

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConsistOfNumberOfLogs", func() {
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
			Expect(fileBytes).Should(ConsistOfNumberOfLogs(25))
		})
	})

})

var _ = Describe("ContainLogs", func() {
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

var _ = Describe("ContainsLogsWith", func() {
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
