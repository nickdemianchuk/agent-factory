package factory

import (
	"regexp"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/google/uuid"
)

func TestNewAgentBoxIDIsTimeOrderedUUIDv7(t *testing.T) {
	g := NewWithT(t)
	crdPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	prev := ""
	for range 100 {
		id := NewAgentBoxID()
		g.Expect(id).To(MatchRegexp(crdPattern.String()))
		parsed, err := uuid.Parse(id)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(parsed.Version()).To(Equal(uuid.Version(7)))
		g.Expect(id > prev).To(BeTrue(), "%s should sort after %s", id, prev)
		prev = id
	}
}
