import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiPlay, FiPause, FiEdit2, FiClock, FiZap } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Modal, Badge } from '@/components/common';
import { useServicesStore } from '@/stores/servicesStore';
import type { AutomationTask } from '@/types';
import toast from 'react-hot-toast';

export function AutomationServicePage() {
  const { automationTasks, fetchAutomationTasks, createAutomationTask, updateAutomationTask, deleteAutomationTask, executeTask } = useServicesStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    triggerType: 'cron' as 'cron' | 'interval' | 'event' | 'condition',
    triggerConfig: '',
    script: '',
  });

  useEffect(() => {
    fetchAutomationTasks();
  }, [fetchAutomationTasks]);

  const handleCreate = async () => {
    if (!formData.name || !formData.script) {
      toast.error('Name and script are required');
      return;
    }

    setIsSubmitting(true);
    try {
      const triggerConfig = formData.triggerConfig ? JSON.parse(formData.triggerConfig) : {};
      const success = await createAutomationTask({
        name: formData.name,
        triggerType: formData.triggerType,
        triggerConfig,
        script: formData.script,
        status: 'active',
      });

      if (success) {
        toast.success('Task created');
        setShowCreateModal(false);
        setFormData({ name: '', triggerType: 'cron', triggerConfig: '', script: '' });
      }
    } catch {
      toast.error('Invalid trigger config JSON');
    }
    setIsSubmitting(false);
  };

  const handleToggle = async (task: AutomationTask) => {
    const newStatus = task.status === 'active' ? 'paused' : 'active';
    const success = await updateAutomationTask(task.id, { status: newStatus });
    if (success) toast.success(`Task ${newStatus}`);
  };

  const handleExecute = async (id: string) => {
    const success = await executeTask(id);
    if (success) toast.success('Task executed');
    else toast.error('Execution failed');
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete "${name}"?`)) return;
    const success = await deleteAutomationTask(id);
    if (success) toast.success('Task deleted');
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active': return <Badge variant="success">Active</Badge>;
      case 'paused': return <Badge variant="warning">Paused</Badge>;
      case 'completed': return <Badge variant="info">Completed</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  const getTriggerIcon = (type: string) => {
    switch (type) {
      case 'cron': return <FiClock className="w-4 h-4" />;
      case 'interval': return <FiClock className="w-4 h-4" />;
      case 'event': return <FiZap className="w-4 h-4" />;
      default: return <FiZap className="w-4 h-4" />;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">Automation Service</h1>
          <p className="text-surface-500 mt-1">
            Schedule and automate on-chain operations
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Create Task
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Tasks</p>
          <p className="text-2xl font-bold text-surface-900">{automationTasks.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Active</p>
          <p className="text-2xl font-bold text-green-600">
            {automationTasks.filter(t => t.status === 'active').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Total Runs</p>
          <p className="text-2xl font-bold text-surface-900">
            {automationTasks.reduce((sum, t) => sum + t.runCount, 0)}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Failed</p>
          <p className="text-2xl font-bold text-red-600">
            {automationTasks.filter(t => t.status === 'failed').length}
          </p>
        </Card>
      </div>

      {/* Tasks List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Automation Tasks</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {automationTasks.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No automation tasks. Create one to get started.
            </div>
          ) : (
            automationTasks.map((task) => (
              <div key={task.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h4 className="font-medium text-surface-900">{task.name}</h4>
                      {getStatusBadge(task.status)}
                      <Badge size="sm" variant="default">
                        {getTriggerIcon(task.triggerType)}
                        <span className="ml-1">{task.triggerType}</span>
                      </Badge>
                    </div>
                    <div className="mt-2 flex items-center gap-4 text-sm text-surface-500">
                      <span>Runs: {task.runCount}</span>
                      {task.lastRunAt && (
                        <span>Last: {new Date(task.lastRunAt).toLocaleString()}</span>
                      )}
                      {task.nextRunAt && (
                        <span>Next: {new Date(task.nextRunAt).toLocaleString()}</span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleExecute(task.id)}
                      className="p-2 text-surface-400 hover:text-primary-600 transition-colors"
                      title="Execute Now"
                    >
                      <FiPlay className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleToggle(task)}
                      className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                      title={task.status === 'active' ? 'Pause' : 'Activate'}
                    >
                      {task.status === 'active' ? <FiPause className="w-4 h-4" /> : <FiPlay className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => handleDelete(task.id, task.name)}
                      className="p-2 text-surface-400 hover:text-red-600 transition-colors"
                      title="Delete"
                    >
                      <FiTrash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Create Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="Create Automation Task"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Task Name"
            placeholder="Daily Price Update"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Trigger Type</label>
            <select
              className="w-full px-4 py-2.5 border border-surface-300 rounded-lg"
              value={formData.triggerType}
              onChange={(e) => setFormData({ ...formData, triggerType: e.target.value as any })}
            >
              <option value="cron">Cron Schedule</option>
              <option value="interval">Fixed Interval</option>
              <option value="event">Event-based</option>
              <option value="condition">Condition-based</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Trigger Config (JSON)</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={2}
              placeholder='{"cron": "0 * * * *"}'
              value={formData.triggerConfig}
              onChange={(e) => setFormData({ ...formData, triggerConfig: e.target.value })}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Script</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={6}
              placeholder="// Your automation script here"
              value={formData.script}
              onChange={(e) => setFormData({ ...formData, script: e.target.value })}
            />
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isSubmitting} className="flex-1">
              Create Task
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
