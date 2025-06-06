package macaroons

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	// testMacHex is a read-only supermacaroon.
	testMacHex = "0201036c6e6402f802030a1011540404373b4d0b3682b15ea7af60c" +
		"c121431383434313932313339323432393138343738341a0f0a076163636" +
		"f756e741204726561641a0f0a0761756374696f6e1204726561641a0d0a0" +
		"561756469741204726561641a0c0a04617574681204726561641a0c0a046" +
		"96e666f1204726561641a100a08696e7369676874731204726561641a100" +
		"a08696e766f696365731204726561641a0f0a046c6f6f701202696e12036" +
		"f75741a100a086d616361726f6f6e1204726561641a0f0a076d657373616" +
		"7651204726561641a100a086f6666636861696e1204726561641a0f0a076" +
		"f6e636861696e1204726561641a0d0a056f726465721204726561641a0d0" +
		"a0570656572731204726561641a0d0a0572617465731204726561641a160" +
		"a0e7265636f6d6d656e646174696f6e1204726561641a0e0a067265706f7" +
		"2741204726561641a130a0b73756767657374696f6e731204726561641a0" +
		"c0a04737761701204726561641a0d0a057465726d7312047265616400000" +
		"6202362c91888e95dfbbf1eb995bd0fef2b549e2de7f4e9fa11aff445273" +
		"60a6caf"
)

// TestSuperMacaroonRootKeyID tests that adding the super macaroon prefix to
// a root key ID results in a valid super macaroon root key ID.
func TestSuperMacaroonRootKeyID(t *testing.T) {
	t.Parallel()

	someBytes := [4]byte{02, 03, 44, 88}
	rootKeyID := NewSuperMacaroonRootKeyID(someBytes)
	require.True(t, isSuperMacaroonRootKeyID(rootKeyID))
	require.False(t, isSuperMacaroonRootKeyID(123))
}

// TestIsSuperMacaroon tests that we can correctly identify an example super
// macaroon.
func TestIsSuperMacaroon(t *testing.T) {
	t.Parallel()

	require.True(t, IsSuperMacaroon(testMacHex))
}
