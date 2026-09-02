// undoable.ts — Generic undo/redo state management for React.
// Uses snapshot-based approach for simplicity and reliability.

export interface UndoableState<T> {
  past: T[];
  present: T;
  future: T[];
}

export interface UndoableActions<T> {
  set: (state: T) => void;
  undo: () => void;
  redo: () => void;
  canUndo: () => boolean;
  canRedo: () => boolean;
  clear: () => void;
  subscribe: (listener: () => void) => () => void;
  getState: () => UndoableState<T>;
}

/**
 * Creates an undoable state wrapper with history management.
 * @param initialState - The initial state
 * @param maxHistory - Maximum number of history entries (default: 50)
 */
export function createUndoable<T>(
  initialState: T,
  maxHistory: number = 50
): [UndoableState<T>, UndoableActions<T>] {
  let state: UndoableState<T> = {
    past: [],
    present: initialState,
    future: [],
  };

  // Notify listeners when state changes
  const listeners = new Set<() => void>();

  const notify = () => {
    listeners.forEach((listener) => listener());
  };

  const subscribe = (listener: () => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  };

  const getState = () => state;

  const actions: UndoableActions<T> = {
    set: (newState: T) => {
      state = {
        past: [...state.past.slice(-maxHistory + 1), state.present],
        present: newState,
        future: [],
      };
      notify();
    },

    undo: () => {
      if (state.past.length === 0) return;

      const previous = state.past[state.past.length - 1];
      const newPast = state.past.slice(0, -1);

      state = {
        past: newPast,
        present: previous,
        future: [state.present, ...state.future],
      };
      notify();
    },

    redo: () => {
      if (state.future.length === 0) return;

      const next = state.future[0];
      const newFuture = state.future.slice(1);

      state = {
        past: [...state.past, state.present],
        present: next,
        future: newFuture,
      };
      notify();
    },

    canUndo: () => state.past.length > 0,
    canRedo: () => state.future.length > 0,

    clear: () => {
      state = {
        past: [],
        present: state.present,
        future: [],
      };
      notify();
    },

    subscribe,

    getState,
  };

  return [state, actions];
}

/**
 * React hook for undoable state management.
 */
export function useUndoable<T>(
  initialState: T,
  maxHistory: number = 50
): [T, (state: T) => void, { undo: () => void; redo: () => void; canUndo: boolean; canRedo: boolean; clear: () => void }] {
  // This is a simplified version for React hooks
  // In practice, you'd use useState/useReducer with the undoable pattern
  const [history, setHistory] = React.useState<UndoableState<T>>({
    past: [],
    present: initialState,
    future: [],
  });

  const set = React.useCallback(
    (newState: T) => {
      setHistory((prev) => ({
        past: [...prev.past.slice(-maxHistory + 1), prev.present],
        present: newState,
        future: [],
      }));
    },
    [maxHistory]
  );

  const undo = React.useCallback(() => {
    setHistory((prev) => {
      if (prev.past.length === 0) return prev;

      const previous = prev.past[prev.past.length - 1];
      const newPast = prev.past.slice(0, -1);

      return {
        past: newPast,
        present: previous,
        future: [prev.present, ...prev.future],
      };
    });
  }, []);

  const redo = React.useCallback(() => {
    setHistory((prev) => {
      if (prev.future.length === 0) return prev;

      const next = prev.future[0];
      const newFuture = prev.future.slice(1);

      return {
        past: [...prev.past, prev.present],
        present: next,
        future: newFuture,
      };
    });
  }, []);

  const clear = React.useCallback(() => {
    setHistory((prev) => ({
      past: [],
      present: prev.present,
      future: [],
    }));
  }, []);

  return [
    history.present,
    set,
    {
      undo,
      redo,
      canUndo: history.past.length > 0,
      canRedo: history.future.length > 0,
      clear,
    },
  ];
}

// Import React for the hook
import React from 'react';
