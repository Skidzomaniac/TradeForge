import React from 'react';

interface State {
  hasError: boolean;
  message: string;
}

// ErrorBoundary catches render errors and shows a friendly fallback.
export class ErrorBoundary extends React.Component<{ children: React.ReactNode }, State> {
  state: State = { hasError: false, message: '' };

  static getDerivedStateFromError(err: Error): State {
    return { hasError: true, message: err.message };
  }

  componentDidCatch(err: Error) {
    // In production this would forward to an error tracker.
    console.error('UI error:', err);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen flex flex-col items-center justify-center p-8 font-mono text-center">
          <div className="text-accent-red text-4xl mb-4 font-mono">!</div>
          <h1 className="text-xl text-gray-200 mb-2">Something went wrong</h1>
          <p className="text-gray-500 mb-6 max-w-md">{this.state.message}</p>
          <button
            onClick={() => this.setState({ hasError: false, message: '' })}
            className="bg-accent-green text-surface-primary font-bold py-2 px-4 rounded"
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
