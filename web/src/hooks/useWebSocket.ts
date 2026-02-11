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
  const pingIntervalRef = useRef<number>();
  const reconnectAttempts = useRef(0);
  const { setWsConnected, setActiveBenchmark, refreshRun } = useStore();

  const connect = useCallback(() => {
    // Clean up any existing connection
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('WebSocket connected');
      setWsConnected(true);
      reconnectAttempts.current = 0;
      
      // Start ping interval to keep connection alive (every 25 seconds)
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current);
      }
      pingIntervalRef.current = window.setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping' }));
        }
      }, 25000);
    };

    ws.onclose = (event) => {
      // Clear ping interval
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current);
        pingIntervalRef.current = undefined;
      }
      
      // Don't log normal closures (1000) or going away (1001)
      if (event.code !== 1000 && event.code !== 1001) {
        console.log('WebSocket disconnected, code:', event.code);
      }
      setWsConnected(false);
      
      // Always retry with exponential backoff
      const delay = Math.min(1000 * Math.pow(1.5, reconnectAttempts.current), 30000);
      reconnectAttempts.current++;
      reconnectTimeoutRef.current = window.setTimeout(() => {
        connect();
      }, delay);
    };

    ws.onerror = () => {
      // Errors are handled by onclose - suppress console noise
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
      // Clean up on unmount
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current);
      }
      if (wsRef.current) {
        // Use code 1000 for normal closure to avoid reconnect
        wsRef.current.close(1000, 'Component unmounting');
        wsRef.current = null;
      }
    };
  }, [connect]);

  return { subscribe, unsubscribe };
}
