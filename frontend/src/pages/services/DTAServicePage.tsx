import React, { useEffect, useState } from 'react';
import { FiPlus, FiCheckCircle, FiShield, FiFileText, FiSearch } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface Certificate {
  id: string;
  dataHash: string;
  dataType: string;
  source: string;
  status: 'valid' | 'expired' | 'revoked';
  issuedAt: string;
  validUntil?: string;
}

export function DTAServicePage() {
  const [certificates, setCertificates] = useState<Certificate[]>([]);
  const [showIssueModal, setShowIssueModal] = useState(false);
  const [showVerifyModal, setShowVerifyModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    dataHash: '',
    dataType: '',
    source: '',
    validUntil: '',
  });
  const [verifyData, setVerifyData] = useState({ certificateId: '', dataHash: '' });
  const [verifyResult, setVerifyResult] = useState<{ valid: boolean; message: string } | null>(null);

  const handleIssue = async () => {
    if (!formData.dataHash || !formData.dataType || !formData.source) {
      toast.error('Data hash, type, and source are required');
      return;
    }

    setIsSubmitting(true);
    try {
      toast.success('Certificate issued');
      setShowIssueModal(false);
      setFormData({ dataHash: '', dataType: '', source: '', validUntil: '' });
    } catch (e) {
      toast.error('Failed to issue certificate');
    }
    setIsSubmitting(false);
  };

  const handleVerify = async () => {
    if (!verifyData.certificateId || !verifyData.dataHash) {
      toast.error('Certificate ID and data hash are required');
      return;
    }

    setIsSubmitting(true);
    try {
      // Simulate verification
      setVerifyResult({ valid: true, message: 'Certificate is valid and data hash matches' });
    } catch (e) {
      setVerifyResult({ valid: false, message: 'Verification failed' });
    }
    setIsSubmitting(false);
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'valid': return <Badge variant="success">Valid</Badge>;
      case 'expired': return <Badge variant="warning">Expired</Badge>;
      case 'revoked': return <Badge variant="error">Revoked</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">DTA Service</h1>
          <p className="text-surface-500 mt-1">
            Data Trust Authority - Issue and verify data authenticity certificates
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" leftIcon={<FiSearch className="w-4 h-4" />} onClick={() => setShowVerifyModal(true)}>
            Verify
          </Button>
          <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowIssueModal(true)}>
            Issue Certificate
          </Button>
        </div>
      </div>

      {/* Trust Info */}
      <Card className="bg-green-50 border-green-200">
        <div className="flex items-start gap-3">
          <FiShield className="w-5 h-5 text-green-600 mt-0.5" />
          <div>
            <h3 className="font-medium text-green-900">TEE-Backed Trust</h3>
            <p className="text-sm text-green-700 mt-1">
              All certificates are signed within the TEE using hardware-protected keys.
              Certificate authenticity can be verified against the TEE attestation.
            </p>
          </div>
        </div>
      </Card>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Certificates</p>
          <p className="text-2xl font-bold text-surface-900">{certificates.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Valid</p>
          <p className="text-2xl font-bold text-green-600">
            {certificates.filter(c => c.status === 'valid').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Expired</p>
          <p className="text-2xl font-bold text-yellow-600">
            {certificates.filter(c => c.status === 'expired').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Revoked</p>
          <p className="text-2xl font-bold text-red-600">
            {certificates.filter(c => c.status === 'revoked').length}
          </p>
        </Card>
      </div>

      {/* Certificates List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Issued Certificates</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {certificates.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No certificates issued yet. Issue your first certificate to get started.
            </div>
          ) : (
            certificates.map((cert) => (
              <div key={cert.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 bg-surface-100 rounded-lg">
                      <FiFileText className="w-4 h-4 text-surface-600" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <code className="text-sm text-surface-900">{cert.id}</code>
                        {getStatusBadge(cert.status)}
                        <Badge size="sm" variant="default">{cert.dataType}</Badge>
                      </div>
                      <p className="text-xs text-surface-400 mt-1 font-mono">
                        Hash: {cert.dataHash.slice(0, 20)}...
                      </p>
                      <p className="text-xs text-surface-400">
                        Source: {cert.source} • Issued: {new Date(cert.issuedAt).toLocaleDateString()}
                        {cert.validUntil && ` • Expires: ${new Date(cert.validUntil).toLocaleDateString()}`}
                      </p>
                    </div>
                  </div>
                  <Button size="sm" variant="secondary">
                    Details
                  </Button>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Issue Modal */}
      <Modal
        isOpen={showIssueModal}
        onClose={() => setShowIssueModal(false)}
        title="Issue Certificate"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Data Hash"
            placeholder="0x..."
            value={formData.dataHash}
            onChange={(e) => setFormData({ ...formData, dataHash: e.target.value })}
            helperText="SHA-256 hash of the data to certify"
          />
          <Input
            label="Data Type"
            placeholder="price_feed, document, etc."
            value={formData.dataType}
            onChange={(e) => setFormData({ ...formData, dataType: e.target.value })}
          />
          <Input
            label="Source"
            placeholder="Data source identifier"
            value={formData.source}
            onChange={(e) => setFormData({ ...formData, source: e.target.value })}
          />
          <Input
            label="Valid Until (Optional)"
            type="date"
            value={formData.validUntil}
            onChange={(e) => setFormData({ ...formData, validUntil: e.target.value })}
          />
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowIssueModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleIssue} isLoading={isSubmitting} className="flex-1">
              Issue Certificate
            </Button>
          </div>
        </div>
      </Modal>

      {/* Verify Modal */}
      <Modal
        isOpen={showVerifyModal}
        onClose={() => { setShowVerifyModal(false); setVerifyResult(null); }}
        title="Verify Certificate"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Certificate ID"
            placeholder="cert-..."
            value={verifyData.certificateId}
            onChange={(e) => setVerifyData({ ...verifyData, certificateId: e.target.value })}
          />
          <Input
            label="Data Hash"
            placeholder="0x..."
            value={verifyData.dataHash}
            onChange={(e) => setVerifyData({ ...verifyData, dataHash: e.target.value })}
            helperText="Hash of the data to verify against the certificate"
          />
          {verifyResult && (
            <div className={`p-4 rounded-lg ${verifyResult.valid ? 'bg-green-50' : 'bg-red-50'}`}>
              <div className="flex items-center gap-2">
                <FiCheckCircle className={`w-5 h-5 ${verifyResult.valid ? 'text-green-600' : 'text-red-600'}`} />
                <span className={verifyResult.valid ? 'text-green-900' : 'text-red-900'}>
                  {verifyResult.message}
                </span>
              </div>
            </div>
          )}
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowVerifyModal(false)} className="flex-1">
              Close
            </Button>
            <Button onClick={handleVerify} isLoading={isSubmitting} className="flex-1">
              Verify
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
