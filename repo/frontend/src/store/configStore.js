import { create } from 'zustand';
import client from '../api/client';

export const useConfigStore = create((set, get) => ({
  configs: [],
  flags: [],
  loading: false,
  error: null,

  fetchConfigs: async () => {
    set({ loading: true, error: null });
    try {
      const res = await client.get('/config/all');
      set({ configs: res.data || [], loading: false });
    } catch (err) {
      set({
        loading: false,
        error: err.response?.data?.message || 'Failed to load configs',
      });
    }
  },

  fetchFlags: async () => {
    set({ loading: true, error: null });
    try {
      const res = await client.get('/config/flags');
      set({ flags: res.data || [], loading: false });
    } catch (err) {
      set({
        loading: false,
        error: err.response?.data?.message || 'Failed to load flags',
      });
    }
  },

  updateConfig: async (key, value) => {
    try {
      await client.put(`/config/${key}`, { value });
      await get().fetchConfigs();
    } catch (err) {
      throw new Error(
        err.response?.data?.message || 'Failed to update config'
      );
    }
  },

  updateFlag: async (key, updates) => {
    try {
      await client.put(`/config/flags/${key}`, updates);
      await get().fetchFlags();
    } catch (err) {
      throw new Error(
        err.response?.data?.message || 'Failed to update flag'
      );
    }
  },

  isFlagEnabled: (key) => {
    const { flags } = get();
    const flag = flags.find((f) => f.key === key);
    return flag?.enabled || false;
  },
}));
