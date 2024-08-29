package conntrack

type Conntrack struct{}

// Not implemented for Windows
func New() *Conntrack {
	return nil
}
