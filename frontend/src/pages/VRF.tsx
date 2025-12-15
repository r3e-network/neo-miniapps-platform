import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  Dices,
  Shield,
  CheckCircle,
  AlertCircle,
  Loader2,
  Copy,
  ExternalLink,
  Hash,
  Clock
} from 'lucide-react';
import { api } from '../api/client';

interface VRFResult {
  request_id: string;
  seed: string;
  random_words: string[];
  proof: string;
  public_key: string;
  timestamp: string;
}

export function VRF() {
  const [seed, setSeed] = useState('');
  const [numWords, setNumWords] = useState(1);
  const [result, setResult] = useState<VRFResult | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [verificationResult, setVerificationResult] = useState<boolean | null>(null);

  // Create VRF request mutation
  const requestMutation = useMutation({
    mutationFn: (data: { seed: string; numWords: number }) =>
      api.neorandRandom(data.seed, data.numWords),
    onSuccess: (data: unknown) => {
      setResult(data as VRFResult);
      setVerificationResult(null);
    },
  });

  // Handle submit
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!seed.trim()) return;
    requestMutation.mutate({ seed, numWords });
  };

  // Verify VRF proof
  const handleVerify = async () => {
    if (!result) return;

    setVerifying(true);
    try {
      const res = await api.neorandVerify(result.seed, result.random_words, result.proof, result.public_key);
      setVerificationResult(Boolean(res?.valid));
    } catch (error) {
      setVerificationResult(false);
    } finally {
      setVerifying(false);
    }
  };

  // Copy to clipboard
  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  // Format hex string
  const formatHex = (hex: string, maxLength = 16) => {
    if (!hex) return '';
    if (hex.length <= maxLength) return hex;
    return `${hex.slice(0, maxLength)}...${hex.slice(-8)}`;
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="flex items-center gap-3 mb-8">
        <div className="p-3 bg-blue-500/20 rounded-xl">
          <Dices className="w-8 h-8 text-blue-500" />
        </div>
        <div>
          <h1 className="text-3xl font-bold text-white">Verifiable Random Function</h1>
          <p className="text-gray-400">Cryptographically Secure Random Numbers with Proofs</p>
        </div>
      </div>

      {/* Info Banner */}
      <div className="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 mb-8">
        <div className="flex items-start gap-3">
          <Shield className="w-5 h-5 text-blue-500 mt-0.5" />
          <div>
            <h3 className="text-blue-400 font-medium mb-1">How VRF Works</h3>
            <p className="text-gray-400 text-sm">
              VRF generates provably random numbers using cryptographic proofs. Each random value comes with
              a proof that can be verified by anyone, ensuring the randomness is fair and tamper-proof.
              Perfect for lotteries, gaming, and any application requiring verifiable randomness.
            </p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Request Form */}
        <div className="lg:col-span-2">
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h2 className="text-xl font-semibold text-white mb-6">Request Random Number</h2>

            <form onSubmit={handleSubmit} className="space-y-6">
              {/* Seed Input */}
              <div>
                <label className="text-sm font-medium text-gray-300 mb-2 block">
                  Seed Value
                </label>
                <input
                  type="text"
                  value={seed}
                  onChange={(e) => setSeed(e.target.value)}
                  placeholder="Enter any string as seed (e.g., lottery-round-123)"
                  className="w-full bg-gray-700 border border-gray-600 rounded-lg px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
                  required
                />
                <p className="text-xs text-gray-500 mt-2">
                  The seed is combined with the TEE's private key to generate a unique random value.
                </p>
              </div>

              {/* Number of Words */}
              <div>
                <label className="text-sm font-medium text-gray-300 mb-2 block">
                  Number of Random Words
                </label>
                <select
                  value={numWords}
                  onChange={(e) => setNumWords(parseInt(e.target.value))}
                  className="w-full bg-gray-700 border border-gray-600 rounded-lg px-4 py-3 text-white focus:outline-none focus:border-blue-500"
                >
                  <option value={1}>1 word (32 bytes)</option>
                  <option value={2}>2 words (64 bytes)</option>
                  <option value={4}>4 words (128 bytes)</option>
                  <option value={8}>8 words (256 bytes)</option>
                </select>
                <p className="text-xs text-gray-500 mt-2">
                  More words provide more random data but cost slightly more gas.
                </p>
              </div>

              {/* Submit Button */}
              <button
                type="submit"
                disabled={!seed.trim() || requestMutation.isPending}
                className="w-full py-4 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
              >
                {requestMutation.isPending ? (
                  <>
                    <Loader2 className="w-5 h-5 animate-spin" />
                    Generating...
                  </>
                ) : (
                  <>
                    <Dices className="w-5 h-5" />
                    Generate Random Number
                  </>
                )}
              </button>

              {requestMutation.isError && (
                <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-2 text-red-400">
                  <AlertCircle className="w-5 h-5" />
                  {(requestMutation.error as Error)?.message || 'Failed to generate random number'}
                </div>
              )}
            </form>
          </div>

          {/* Result Display */}
          {result && (
            <div className="mt-8 bg-gray-800 rounded-xl border border-gray-700 p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-semibold text-white">Random Number Result</h2>
                <div className="flex items-center gap-2">
                  {verificationResult === true && (
                    <span className="flex items-center gap-1 text-green-400 text-sm">
                      <CheckCircle className="w-4 h-4" />
                      Verified
                    </span>
                  )}
                  {verificationResult === false && (
                    <span className="flex items-center gap-1 text-red-400 text-sm">
                      <AlertCircle className="w-4 h-4" />
                      Verification Failed
                    </span>
                  )}
                </div>
              </div>

              <div className="space-y-4">
                {/* Request ID */}
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Request ID</label>
                  <div className="flex items-center gap-2 bg-gray-700/50 rounded-lg px-4 py-3">
                    <span className="text-white font-mono text-sm flex-1">
                      {result.request_id}
                    </span>
                    <button
                      onClick={() => copyToClipboard(result.request_id)}
                      className="text-gray-400 hover:text-white transition-colors"
                    >
                      <Copy className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Random Words */}
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">
                    Random Words ({result.random_words?.length ?? 0})
                  </label>
                  <div className="flex items-center gap-2 bg-gray-700/50 rounded-lg px-4 py-3">
                    <Hash className="w-5 h-5 text-blue-400" />
                    <div className="text-white font-mono text-sm flex-1 break-all space-y-1">
                      {(result.random_words ?? []).map((word, idx) => (
                        <div key={idx}>{word}</div>
                      ))}
                    </div>
                    <button
                      onClick={() => copyToClipboard(JSON.stringify(result.random_words ?? []))}
                      className="text-gray-400 hover:text-white transition-colors"
                    >
                      <Copy className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Proof */}
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">VRF Proof</label>
                  <div className="flex items-center gap-2 bg-gray-700/50 rounded-lg px-4 py-3">
                    <Shield className="w-5 h-5 text-green-400" />
                    <span className="text-white font-mono text-sm flex-1">
                      {formatHex(result.proof, 32)}
                    </span>
                    <button
                      onClick={() => copyToClipboard(result.proof)}
                      className="text-gray-400 hover:text-white transition-colors"
                    >
                      <Copy className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Public Key */}
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Public Key</label>
                  <div className="flex items-center gap-2 bg-gray-700/50 rounded-lg px-4 py-3">
                    <span className="text-white font-mono text-sm flex-1">
                      {formatHex(result.public_key, 32)}
                    </span>
                    <button
                      onClick={() => copyToClipboard(result.public_key)}
                      className="text-gray-400 hover:text-white transition-colors"
                    >
                      <Copy className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Timestamp */}
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Generated At</label>
                  <div className="flex items-center gap-2 bg-gray-700/50 rounded-lg px-4 py-3">
                    <Clock className="w-5 h-5 text-gray-400" />
                    <span className="text-white text-sm">
                      {new Date(result.timestamp).toLocaleString()}
                    </span>
                  </div>
                </div>
              </div>

              {/* Verify Button */}
              <button
                onClick={handleVerify}
                disabled={verifying || verificationResult === true}
                className="w-full mt-6 py-3 bg-green-600 hover:bg-green-500 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
              >
                {verifying ? (
                  <>
                    <Loader2 className="w-5 h-5 animate-spin" />
                    Verifying Proof...
                  </>
                ) : verificationResult === true ? (
                  <>
                    <CheckCircle className="w-5 h-5" />
                    Proof Verified
                  </>
                ) : (
                  <>
                    <Shield className="w-5 h-5" />
                    Verify Proof
                  </>
                )}
              </button>
            </div>
          )}
        </div>

        {/* Info Sidebar */}
        <div className="space-y-6">
          {/* Use Cases */}
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h3 className="text-lg font-semibold text-white mb-4">Use Cases</h3>
            <ul className="space-y-3 text-sm">
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">Lottery and gaming systems</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">NFT trait generation</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">Random selection and sampling</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">Cryptographic key generation</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">Fair distribution mechanisms</span>
              </li>
            </ul>
          </div>

          {/* Security Features */}
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h3 className="text-lg font-semibold text-white mb-4">Security Features</h3>
            <ul className="space-y-3 text-sm">
              <li className="flex items-start gap-2">
                <Shield className="w-4 h-4 text-blue-400 mt-0.5" />
                <span className="text-gray-300">TEE-protected private key</span>
              </li>
              <li className="flex items-start gap-2">
                <Shield className="w-4 h-4 text-blue-400 mt-0.5" />
                <span className="text-gray-300">Cryptographic proof generation</span>
              </li>
              <li className="flex items-start gap-2">
                <Shield className="w-4 h-4 text-blue-400 mt-0.5" />
                <span className="text-gray-300">Public verifiability</span>
              </li>
              <li className="flex items-start gap-2">
                <Shield className="w-4 h-4 text-blue-400 mt-0.5" />
                <span className="text-gray-300">Deterministic from seed</span>
              </li>
              <li className="flex items-start gap-2">
                <Shield className="w-4 h-4 text-blue-400 mt-0.5" />
                <span className="text-gray-300">Tamper-proof results</span>
              </li>
            </ul>
          </div>

          {/* Documentation */}
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h3 className="text-lg font-semibold text-white mb-4">Documentation</h3>
            <div className="space-y-3">
              <a
                href="#"
                className="flex items-center gap-2 text-blue-400 hover:text-blue-300 text-sm"
              >
                <ExternalLink className="w-4 h-4" />
                VRF API Reference
              </a>
              <a
                href="#"
                className="flex items-center gap-2 text-blue-400 hover:text-blue-300 text-sm"
              >
                <ExternalLink className="w-4 h-4" />
                Integration Guide
              </a>
              <a
                href="#"
                className="flex items-center gap-2 text-blue-400 hover:text-blue-300 text-sm"
              >
                <ExternalLink className="w-4 h-4" />
                Verification Examples
              </a>
            </div>
          </div>
        </div>
      </div>

      {/* How It Works */}
      <div className="mt-8 bg-gray-800 rounded-xl border border-gray-700 p-6">
        <h2 className="text-xl font-semibold text-white mb-6">How VRF Works</h2>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="text-center">
            <div className="w-12 h-12 bg-blue-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-blue-400 font-bold">1</span>
            </div>
            <h4 className="text-white font-medium mb-1">Submit Seed</h4>
            <p className="text-gray-400 text-sm">Provide a seed value for randomness generation.</p>
          </div>
          <div className="text-center">
            <div className="w-12 h-12 bg-blue-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-blue-400 font-bold">2</span>
            </div>
            <h4 className="text-white font-medium mb-1">TEE Processing</h4>
            <p className="text-gray-400 text-sm">TEE combines seed with private key to generate random value.</p>
          </div>
          <div className="text-center">
            <div className="w-12 h-12 bg-blue-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-blue-400 font-bold">3</span>
            </div>
            <h4 className="text-white font-medium mb-1">Proof Generation</h4>
            <p className="text-gray-400 text-sm">Cryptographic proof is generated for the random value.</p>
          </div>
          <div className="text-center">
            <div className="w-12 h-12 bg-blue-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-blue-400 font-bold">4</span>
            </div>
            <h4 className="text-white font-medium mb-1">Verification</h4>
            <p className="text-gray-400 text-sm">Anyone can verify the proof using the public key.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
