import { useEffect, useRef, useCallback, useState } from 'react';
import { useStore } from '@/store';
import { WSMessage } from '@/types';

// Hook for subscribing to specific benchmark updates
export default function useWebSocket(benchmarkId: string | null) {
  const [lastMessage, setLastMessage] = useState<{
    type: string;
    progress?: number;
    error?: string;
    data?: unknown;
  } | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!benchmarkId) {
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?benchmark=${benchmarkId}`;

    const ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        setLastMessage(data);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    wsRef.current = ws;

    return () => {
      ws.close();
    };
  }, [benchmarkId]);

  return { lastMessage };
}

// Global WebSocket hook for app-wide updates
export function useGlobalWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number>();
  const reconnectAttempts = useRef(0);
  const { setWsConnected, setActiveBenchmark, refreshRun } = useStore();

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('WebSocket connected');
      setWsConnected(true);
      reconnectAttempts.current = 0; // Reset on successful connection
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setWsConnected(false);
      // Exponential backoff with max 30 second delay, max 5 attempts
      if (reconnectAttempts.current < 5) {
        const delay = Math.min(3000 * Math.pow(2, reconnectAttempts.current), 30000);
        reconnectAttempts.current++;
        reconnectTimeoutRef.current = window.setTimeout(() => {
          connect();
        }, delay);
      }
    };

    ws.onerror = () => {
      // Suppress error logging - connection failures are handled by onclose
    };

    ws.onmessage = (event) => {
      try {
        const message: WSMessage = JSON.parse(event.data);
        handleMessage(message);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    wsRef.current = ws;
  }, [setWsConnected]);

  const handleMessage = useCallback((message: WSMessage) => {
    switch (message.type) {
      case 'benchmark_started':
        setActiveBenchmark({
          id: message.benchmark_id,
          status: 'running',
          progress: 0,
        });
        break;

      case 'benchmark_progress':
        const progress = message.data as { progress: number };
        setActiveBenchmark({
          id: message.benchmark_id,
          status: 'running',
          progress: progress.progress,
        });
        break;

      case 'benchmark_completed':
        setActiveBenchmark(null);
        refreshRun(message.benchmark_id);
        break;

      case 'benchmark_failed':
        setActiveBenchmark(null);
        refreshRun(message.benchmark_id);
        break;

      case 'metrics':
        // Real-time metrics update
        // Could be used for live charts
        break;
    }
  }, [setActiveBenchmark, refreshRun]);

  const subscribe = useCallback((benchmarkId: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        action: 'subscribe',
        benchmark_id: benchmarkId,
      }));
    }
  }, []);

  const unsubscribe = useCallback((benchmarkId: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        action: 'unsubscribe',
        benchmark_id: benchmarkId,
      }));
    }
  }, []);

  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [connect]);

  return { subscribe, unsubscribe };
}
