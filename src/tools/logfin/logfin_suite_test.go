package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogfin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logfin Suite")
}
