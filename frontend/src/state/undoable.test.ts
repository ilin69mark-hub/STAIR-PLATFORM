import { renderHook, act } from '@testing-library/react';
import { useUndoable } from './undoable';

describe('useUndoable', () => {
  it('should initialize with initial state', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    const [state, , { canUndo, canRedo }] = result.current;
    expect(state).toBe('initial');
    expect(canUndo).toBe(false);
    expect(canRedo).toBe(false);
  });

  it('should update state', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    const [state] = result.current;
    expect(state).toBe('updated');
  });

  it('should allow undo after state change', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    const [, , { canUndo }] = result.current;
    expect(canUndo).toBe(true);
  });

  it('should undo to previous state', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    act(() => {
      const [, , { undo }] = result.current;
      undo();
    });

    const [state] = result.current;
    expect(state).toBe('initial');
  });

  it('should allow redo after undo', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    act(() => {
      const [, , { undo }] = result.current;
      undo();
    });

    const [, , { canRedo }] = result.current;
    expect(canRedo).toBe(true);
  });

  it('should redo to next state', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    act(() => {
      const [, , { undo }] = result.current;
      undo();
    });

    act(() => {
      const [, , { redo }] = result.current;
      redo();
    });

    const [state] = result.current;
    expect(state).toBe('updated');
  });

  it('should clear future on new state', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    act(() => {
      const [, , { undo }] = result.current;
      undo();
    });

    act(() => {
      const [, set] = result.current;
      set('new');
    });

    const [, , { canRedo }] = result.current;
    expect(canRedo).toBe(false);
  });

  it('should respect max history', () => {
    const { result } = renderHook(() => useUndoable('initial', 3));

    act(() => {
      const [, set] = result.current;
      set('1');
    });

    act(() => {
      const [, set] = result.current;
      set('2');
    });

    act(() => {
      const [, set] = result.current;
      set('3');
    });

    act(() => {
      const [, set] = result.current;
      set('4');
    });

    // Should have 3 past states (1, 2, 3), not 4
    const [state, , { canUndo }] = result.current;
    expect(state).toBe('4');
    expect(canUndo).toBe(true);
  });

  it('should clear history', () => {
    const { result } = renderHook(() => useUndoable('initial'));

    act(() => {
      const [, set] = result.current;
      set('updated');
    });

    act(() => {
      const [, , { clear }] = result.current;
      clear();
    });

    const [, , { canUndo, canRedo }] = result.current;
    expect(canUndo).toBe(false);
    expect(canRedo).toBe(false);
  });
});
