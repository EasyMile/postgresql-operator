package postgresql

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("PostgresqlUser tests", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		// TODO
	})

	It("should fail to look a not found pgdb", func() {
		// TODO
	})

	It("should be ok to set only required values", func() {
		// TODO
	})

	It("should be ok to set all values (required & optional)", func() {
		// TODO
	})

	It("should be ok to change role prefix", func() {
		// TODO
	})

	It("should be ok to change privileges (OWNER -> READ)", func() {
		// TODO
	})

	It("should be ok to regenerate a secret that have been removed", func() {
		// TODO
	})

	It("should be ok to regenerate a secret that have been edited (key removed)", func() {
		// TODO
	})

	It("should be ok to regenerate a secret that been edited (known field edited)", func() {
		// TODO
	})

	It("should be ok to remove an external user", func() {
		// TODO
	})
})
