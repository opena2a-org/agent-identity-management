// Package ed25519keytest finds Ed25519 public-key encodings for tests. It is
// not used by production code.
package ed25519keytest

import (
	"math/big"
)

var (
	p = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	d = func() *big.Int {
		num := new(big.Int).Neg(big.NewInt(121665))
		den := new(big.Int).ModInverse(big.NewInt(121666), p)
		return num.Mul(num, den).Mod(num, p)
	}()
)

// decodes reports whether y has an x on the curve, by the decoding RFC 8032
// describes: take the candidate root x = (u/v)^((p+3)/8) and, if v*x^2 is -u,
// multiply it by sqrt(-1); the y has no point if neither candidate fits.
// OnCurve reports whether y has a matching x on the curve.
func OnCurve(y *big.Int) bool { return decodes(y) }

func decodes(y *big.Int) bool {
	y2 := new(big.Int).Mul(y, y)
	u := new(big.Int).Sub(y2, big.NewInt(1))
	u.Mod(u, p)
	v := new(big.Int).Mul(d, y2)
	v.Add(v, big.NewInt(1)).Mod(v, p)
	x := new(big.Int).ModInverse(v, p)
	x.Mul(x, u).Mod(x, p)
	e := new(big.Int).Add(p, big.NewInt(3))
	e.Rsh(e, 3)
	x.Exp(x, e, p)
	vx2 := new(big.Int).Mul(x, x)
	vx2.Mul(vx2, v).Mod(vx2, p)
	if vx2.Cmp(u) == 0 {
		return true
	}
	negU := new(big.Int).Sub(p, u)
	negU.Mod(negU, p)
	return vx2.Cmp(negU) == 0
}

// OffCurve returns the little-endian encoding of the first canonical y (from 2
// up) that has no matching x on the curve: 32 bytes that are below p and not
// small-order, but not a point.
func OffCurve() []byte {
	for n := int64(2); ; n++ {
		y := big.NewInt(n)
		if decodes(y) {
			continue
		}
		be := y.FillBytes(make([]byte, 32))
		out := make([]byte, 32)
		for i := range out {
			out[i] = be[31-i]
		}
		return out
	}
}
