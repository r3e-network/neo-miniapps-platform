import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrendingUp, FiTrendingDown, FiMinus, FiRefreshCw } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Modal, Badge } from '@/components/common';
import { useServicesStore } from '@/stores/servicesStore';
import type { PriceFeed } from '@/types';
import toast from 'react-hot-toast';

export function DataFeedsServicePage() {
  const { priceFeeds, fetchPriceFeeds, createPriceFeed, updatePriceFeed } = useServicesStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    symbol: '',
    sources: '',
    updateInterval: '60',
    deviationThreshold: '0.5',
  });

  useEffect(() => {
    fetchPriceFeeds();
    const interval = setInterval(fetchPriceFeeds, 30000);
    return () => clearInterval(interval);
  }, [fetchPriceFeeds]);

  const handleCreate = async () => {
    if (!formData.symbol || !formData.sources) {
      toast.error('Symbol and sources are required');
      return;
    }

    setIsSubmitting(true);
    const success = await createPriceFeed({
      symbol: formData.symbol.toUpperCase(),
      sources: formData.sources.split(',').map(s => s.trim()),
      updateInterval: parseInt(formData.updateInterval),
      deviationThreshold: parseFloat(formData.deviationThreshold),
      enabled: true,
    });

    if (success) {
      toast.success('Price feed created');
      setShowCreateModal(false);
      setFormData({ symbol: '', sources: '', updateInterval: '60', deviationThreshold: '0.5' });
    } else {
      toast.error('Failed to create price feed');
    }
    setIsSubmitting(false);
  };

  const handleToggle = async (feed: PriceFeed) => {
    const success = await updatePriceFeed(feed.id, { enabled: !feed.enabled });
    if (success) {
      toast.success(feed.enabled ? 'Feed paused' : 'Feed activated');
    }
  };

  const formatPrice = (price?: number) => {
    if (!price) return '-';
    return price.toLocaleString('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    });
  };

  const getPriceChangeIcon = (symbol: string) => {
    // Mock price change - in real app would come from data
    const change = Math.random() > 0.5 ? 1 : -1;
    if (change > 0) return <FiTrendingUp className="w-4 h-4 text-green-500" />;
    if (change < 0) return <FiTrendingDown className="w-4 h-4 text-red-500" />;
    return <FiMinus className="w-4 h-4 text-surface-400" />;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">DataFeeds Service</h1>
          <p className="text-surface-500 mt-1">
            Real-time price feeds with multi-source aggregation
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            leftIcon={<FiRefreshCw className="w-4 h-4" />}
            onClick={() => fetchPriceFeeds()}
          >
            Refresh
          </Button>
          <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
            Add Price Feed
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Feeds</p>
          <p className="text-2xl font-bold text-surface-900">{priceFeeds.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Active</p>
          <p className="text-2xl font-bold text-green-600">
            {priceFeeds.filter(f => f.enabled).length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Update Interval</p>
          <p className="text-2xl font-bold text-surface-900">60s</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Sources</p>
          <p className="text-2xl font-bold text-surface-900">
            {new Set(priceFeeds.flatMap(f => f.sources)).size}
          </p>
        </Card>
      </div>

      {/* Price Feeds Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {priceFeeds.length === 0 ? (
          <Card className="col-span-full">
            <div className="text-center py-8 text-surface-400">
              No price feeds configured. Add one to get started.
            </div>
          </Card>
        ) : (
          priceFeeds.map((feed) => (
            <Card key={feed.id} hover>
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="text-lg font-bold text-surface-900">{feed.symbol}</h3>
                  <p className="text-xs text-surface-400">
                    {feed.sources.length} source{feed.sources.length > 1 ? 's' : ''}
                  </p>
                </div>
                <Badge variant={feed.enabled ? 'success' : 'warning'}>
                  {feed.enabled ? 'Active' : 'Paused'}
                </Badge>
              </div>

              <div className="flex items-end justify-between">
                <div>
                  <p className="text-2xl font-bold text-surface-900">
                    {formatPrice(feed.lastPrice)}
                  </p>
                  <div className="flex items-center gap-1 mt-1">
                    {getPriceChangeIcon(feed.symbol)}
                    <span className="text-sm text-surface-500">
                      Updated {feed.lastUpdatedAt
                        ? new Date(feed.lastUpdatedAt).toLocaleTimeString()
                        : 'Never'}
                    </span>
                  </div>
                </div>
              </div>

              <div className="mt-4 pt-4 border-t border-surface-200">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-surface-500">Interval</span>
                  <span className="text-surface-900">{feed.updateInterval}s</span>
                </div>
                <div className="flex items-center justify-between text-sm mt-1">
                  <span className="text-surface-500">Deviation</span>
                  <span className="text-surface-900">{feed.deviationThreshold}%</span>
                </div>
              </div>

              <div className="mt-4 flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  className="flex-1"
                  onClick={() => handleToggle(feed)}
                >
                  {feed.enabled ? 'Pause' : 'Activate'}
                </Button>
              </div>
            </Card>
          ))
        )}
      </div>

      {/* Create Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="Add Price Feed"
      >
        <div className="space-y-4">
          <Input
            label="Symbol"
            placeholder="BTC/USD"
            value={formData.symbol}
            onChange={(e) => setFormData({ ...formData, symbol: e.target.value })}
          />
          <Input
            label="Sources"
            placeholder="binance, coinbase, kraken"
            value={formData.sources}
            onChange={(e) => setFormData({ ...formData, sources: e.target.value })}
            helperText="Comma-separated list of price sources"
          />
          <Input
            label="Update Interval (seconds)"
            type="number"
            value={formData.updateInterval}
            onChange={(e) => setFormData({ ...formData, updateInterval: e.target.value })}
          />
          <Input
            label="Deviation Threshold (%)"
            type="number"
            step="0.1"
            value={formData.deviationThreshold}
            onChange={(e) => setFormData({ ...formData, deviationThreshold: e.target.value })}
            helperText="Minimum price change to trigger update"
          />
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isSubmitting} className="flex-1">
              Create
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
