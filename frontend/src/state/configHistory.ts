// configHistory.ts — Undo/Redo for stair configuration edits.
import { useUndoable } from './undoable';

export interface StairConfig {
  id?: string;
  projectId?: string;
  name: string;
  width: number;
  height: number;
  flightType: string;
  stepCount?: number;
  stepHeight?: number;
  treadDepth?: number;
  stringerThickness?: number;
  stepThickness?: number;
}

const defaultConfig: StairConfig = {
  name: '',
  width: 900,
  height: 2700,
  flightType: 'straight',
  stepHeight: 180,
  stringerThickness: 50,
  stepThickness: 40,
};

/**
 * Hook for managing undoable stair configuration state.
 */
export function useConfigHistory(initialConfig?: StairConfig) {
  const [config, setConfig, { undo, redo, canUndo, canRedo }] = useUndoable<StairConfig>(
    initialConfig || defaultConfig,
    50 // max 50 history entries
  );

  const updateConfig = (updates: Partial<StairConfig>) => {
    setConfig({ ...config, ...updates });
  };

  const resetConfig = () => {
    setConfig(defaultConfig);
  };

  return {
    config,
    setConfig,
    updateConfig,
    resetConfig,
    undo,
    redo,
    canUndo,
    canRedo,
  };
}
