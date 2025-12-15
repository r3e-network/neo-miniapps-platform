package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"math/big"
)

// =============================================================================
// ECVRF-P256-SHA256-TAI Implementation (RFC 9381)
// =============================================================================
//
// This implements a proper Verifiable Random Function (VRF) that provides:
// 1. Determinism: Same input always produces same output
// 2. Unpredictability: Without private key, output cannot be predicted
// 3. Verifiability: Anyone with public key can verify the proof
//
// The implementation follows RFC 9381 ECVRF-P256-SHA256-TAI suite.

// VRFOutput represents the output of a VRF computation.
type VRFOutput struct {
	// Beta is the VRF output (32 bytes hash)
	Beta []byte
	// Pi is the VRF proof (Gamma point + c scalar + s scalar)
	Pi *VRFProofData
}

// VRFProofData contains the proof components.
type VRFProofData struct {
	// Gamma is the VRF output point (before hashing)
	GammaX, GammaY *big.Int
	// c is the challenge scalar
	C *big.Int
	// s is the response scalar
	S *big.Int
}

// Constants for ECVRF-P256-SHA256-TAI
var (
	// Suite string for P256-SHA256-TAI
	vrfSuiteString = []byte{0x01} // ECVRF-P256-SHA256-TAI

	// Field prime for P256
	p256 = elliptic.P256()
)

// GenerateVRFProof generates a VRF proof for the given input (alpha).
// This is a deterministic function - same key and alpha always produce same output.
func GenerateVRFProof(privateKey *ecdsa.PrivateKey, alpha []byte) (*VRFOutput, error) {
	if privateKey == nil {
		return nil, errors.New("private key is nil")
	}

	curve := privateKey.Curve
	if curve != p256 {
		return nil, errors.New("only P-256 curve is supported")
	}

	// Step 1: Hash alpha to a curve point H
	hX, hY, err := hashToCurveP256(alpha, &privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	// Step 2: Compute Gamma = x * H (where x is the private key)
	gammaX, gammaY := curve.ScalarMult(hX, hY, privateKey.D.Bytes())

	// Step 3: Generate deterministic nonce k using RFC 6979
	k := generateDeterministicK(privateKey, hX, hY)

	// Step 4: Compute U = k * G (generator point)
	uX, uY := curve.ScalarBaseMult(k.Bytes())

	// Step 5: Compute V = k * H
	vX, vY := curve.ScalarMult(hX, hY, k.Bytes())

	// Step 6: Compute challenge c = ECVRF_challenge_generation(Y, H, Gamma, U, V)
	c := computeChallenge(curve, &privateKey.PublicKey, hX, hY, gammaX, gammaY, uX, uY, vX, vY)

	// Step 7: Compute s = (k + c * x) mod n
	n := curve.Params().N
	cx := new(big.Int).Mul(c, privateKey.D)
	cx.Mod(cx, n)
	s := new(big.Int).Add(k, cx)
	s.Mod(s, n)

	// Step 8: Compute beta = ECVRF_proof_to_hash(Gamma)
	beta := proofToHash(gammaX, gammaY)

	return &VRFOutput{
		Beta: beta,
		Pi: &VRFProofData{
			GammaX: gammaX,
			GammaY: gammaY,
			C:      c,
			S:      s,
		},
	}, nil
}

// VerifyVRFProof verifies a VRF proof and returns the output if valid.
func VerifyVRFProof(publicKey *ecdsa.PublicKey, alpha []byte, proof *VRFProofData) ([]byte, bool) {
	if publicKey == nil || proof == nil {
		return nil, false
	}

	curve := publicKey.Curve
	if curve != p256 {
		return nil, false
	}

	// Verify Gamma is on the curve
	if !curve.IsOnCurve(proof.GammaX, proof.GammaY) {
		return nil, false
	}

	// Step 1: Hash alpha to curve point H
	hX, hY, err := hashToCurveP256(alpha, publicKey)
	if err != nil {
		return nil, false
	}

	// Step 2: Compute U = s*G - c*Y
	// U = s*G + (-c)*Y
	n := curve.Params().N
	negC := new(big.Int).Neg(proof.C)
	negC.Mod(negC, n)

	sGx, sGy := curve.ScalarBaseMult(proof.S.Bytes())
	cYx, cYy := curve.ScalarMult(publicKey.X, publicKey.Y, negC.Bytes())
	uX, uY := curve.Add(sGx, sGy, cYx, cYy)

	// Step 3: Compute V = s*H - c*Gamma
	sHx, sHy := curve.ScalarMult(hX, hY, proof.S.Bytes())
	cGammaX, cGammaY := curve.ScalarMult(proof.GammaX, proof.GammaY, negC.Bytes())
	vX, vY := curve.Add(sHx, sHy, cGammaX, cGammaY)

	// Step 4: Compute expected challenge c'
	cPrime := computeChallenge(curve, publicKey, hX, hY, proof.GammaX, proof.GammaY, uX, uY, vX, vY)

	// Step 5: Verify c == c'
	if proof.C.Cmp(cPrime) != 0 {
		return nil, false
	}

	// Step 6: Compute beta = ECVRF_proof_to_hash(Gamma)
	beta := proofToHash(proof.GammaX, proof.GammaY)

	return beta, true
}

// hashToCurveP256 implements the try-and-increment method for P-256.
// This is the TAI (Try-And-Increment) method from RFC 9381.
func hashToCurveP256(alpha []byte, publicKey *ecdsa.PublicKey) (x, y *big.Int, err error) {
	curve := p256
	params := curve.Params()

	// Encode public key
	pkBytes := elliptic.MarshalCompressed(curve, publicKey.X, publicKey.Y)

	// Try-and-increment: try different counter values until we find a valid point
	for ctr := byte(0); ctr < 255; ctr++ {
		// hash_string is the concatenation of suite_string, 0x01, pk, alpha, and ctr.
		h := sha256.New()
		h.Write(vrfSuiteString)
		h.Write([]byte{0x01}) // hash_to_curve domain separator
		h.Write(pkBytes)
		h.Write(alpha)
		h.Write([]byte{ctr})
		hashValue := h.Sum(nil)

		// Interpret hash as x-coordinate candidate
		xCandidate := new(big.Int).SetBytes(hashValue)
		xCandidate.Mod(xCandidate, params.P)

		// Try to find y such that (x, y) is on the curve
		// y^2 = x^3 - 3x + b (mod p) for P-256
		yCandidate := computeYFromX(curve, xCandidate)
		if yCandidate != nil {
			// Ensure we always use the even y (for determinism)
			if yCandidate.Bit(0) == 1 {
				yCandidate.Sub(params.P, yCandidate)
			}

			// Verify point is on curve
			if curve.IsOnCurve(xCandidate, yCandidate) {
				return xCandidate, yCandidate, nil
			}
		}
	}

	return nil, nil, errors.New("failed to hash to curve after 255 attempts")
}

// computeYFromX computes y from x for the P-256 curve.
// Returns nil if no valid y exists.
func computeYFromX(curve elliptic.Curve, x *big.Int) *big.Int {
	params := curve.Params()
	p := params.P

	// y^2 = x^3 - 3x + b (mod p)
	// For P-256: a = -3, b is the curve parameter

	// x^3
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Mod(x3, p)

	// -3x
	threeX := new(big.Int).Mul(big.NewInt(3), x)
	threeX.Mod(threeX, p)

	// x^3 - 3x
	y2 := new(big.Int).Sub(x3, threeX)
	y2.Mod(y2, p)
	if y2.Sign() < 0 {
		y2.Add(y2, p)
	}

	// x^3 - 3x + b
	y2.Add(y2, params.B)
	y2.Mod(y2, p)

	// Compute square root using Tonelli-Shanks (P-256 is p ≡ 3 mod 4)
	// y = y2^((p+1)/4) mod p
	exp := new(big.Int).Add(p, big.NewInt(1))
	exp.Div(exp, big.NewInt(4))
	y := new(big.Int).Exp(y2, exp, p)

	// Verify: y^2 == y2 (mod p)
	ySquared := new(big.Int).Mul(y, y)
	ySquared.Mod(ySquared, p)
	if ySquared.Cmp(y2) != 0 {
		return nil // No valid y exists
	}

	return y
}

// generateDeterministicK generates a deterministic nonce k using HMAC-DRBG (RFC 6979 style).
func generateDeterministicK(privateKey *ecdsa.PrivateKey, hX, hY *big.Int) *big.Int {
	curve := privateKey.Curve
	n := curve.Params().N

	// Create deterministic input: private_key || H_x || H_y
	h := hmac.New(sha256.New, privateKey.D.Bytes())
	h.Write(hX.Bytes())
	h.Write(hY.Bytes())
	kBytes := h.Sum(nil)

	k := new(big.Int).SetBytes(kBytes)
	k.Mod(k, n)

	// Ensure k is not zero
	if k.Sign() == 0 {
		k.SetInt64(1)
	}

	return k
}

// computeChallenge computes the challenge scalar c.
func computeChallenge(curve elliptic.Curve, publicKey *ecdsa.PublicKey,
	hX, hY, gammaX, gammaY, uX, uY, vX, vY *big.Int) *big.Int {

	n := curve.Params().N

	// Encode all points
	h := sha256.New()
	h.Write(vrfSuiteString)
	h.Write([]byte{0x02}) // challenge domain separator

	// Encode public key Y
	h.Write(elliptic.MarshalCompressed(curve, publicKey.X, publicKey.Y))

	// Encode H
	h.Write(elliptic.MarshalCompressed(curve, hX, hY))

	// Encode Gamma
	h.Write(elliptic.MarshalCompressed(curve, gammaX, gammaY))

	// Encode U
	h.Write(elliptic.MarshalCompressed(curve, uX, uY))

	// Encode V
	h.Write(elliptic.MarshalCompressed(curve, vX, vY))

	// Take first 16 bytes (128 bits) as per RFC 9381
	hashValue := h.Sum(nil)
	c := new(big.Int).SetBytes(hashValue[:16])
	c.Mod(c, n)

	return c
}

// proofToHash converts the Gamma point to the VRF output beta.
func proofToHash(gammaX, gammaY *big.Int) []byte {
	h := sha256.New()
	h.Write(vrfSuiteString)
	h.Write([]byte{0x03}) // proof_to_hash domain separator

	// Multiply Gamma by cofactor (1 for P-256)
	// Encode Gamma
	h.Write(elliptic.MarshalCompressed(p256, gammaX, gammaY))

	return h.Sum(nil)
}

// =============================================================================
// Serialization Helpers
// =============================================================================

// SerializeVRFProof serializes a VRF proof to bytes.
// Format: Gamma (33 bytes compressed) || c (32 bytes) || s (32 bytes) = 97 bytes
func SerializeVRFProof(proof *VRFProofData) []byte {
	if proof == nil {
		return nil
	}

	result := make([]byte, 97)

	// Gamma (compressed point, 33 bytes)
	gamma := elliptic.MarshalCompressed(p256, proof.GammaX, proof.GammaY)
	copy(result[0:33], gamma)

	// c (32 bytes, big-endian, zero-padded)
	cBytes := proof.C.Bytes()
	copy(result[33+(32-len(cBytes)):65], cBytes)

	// s (32 bytes, big-endian, zero-padded)
	sBytes := proof.S.Bytes()
	copy(result[65+(32-len(sBytes)):97], sBytes)

	return result
}

// DeserializeVRFProof deserializes a VRF proof from bytes.
func DeserializeVRFProof(data []byte) (*VRFProofData, error) {
	if len(data) != 97 {
		return nil, errors.New("invalid proof length")
	}

	// Gamma (compressed point)
	gammaX, gammaY := elliptic.UnmarshalCompressed(p256, data[0:33])
	if gammaX == nil {
		return nil, errors.New("invalid Gamma point")
	}

	// c
	c := new(big.Int).SetBytes(data[33:65])

	// s
	s := new(big.Int).SetBytes(data[65:97])

	return &VRFProofData{
		GammaX: gammaX,
		GammaY: gammaY,
		C:      c,
		S:      s,
	}, nil
}
