import { useMemo } from 'react';
import { CallRoom } from './components/CallRoom';

function App() {
  // Read room ID from URL params
  const roomId = useMemo(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get('id');
  }, []);

  // No room ID - show error
  if (!roomId) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center max-w-md">
          <div className="w-16 h-16 mx-auto mb-6 rounded-full bg-parch-light-red/15 border-2 border-parch-light-red/30 flex items-center justify-center">
            <svg className="w-8 h-8 text-parch-light-red" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <h1 className="text-2xl sm:text-3xl font-serif font-bold text-parch-bright-white mb-3 tracking-parch">
            No room specified
          </h1>
          <p className="text-parch-gray mb-6 tracking-parch">
            Please use a valid call link or go back to the landing page
          </p>
          <a
            href="/call"
            className="parch-btn inline-block bg-parch-light-blue text-parch-bright-white font-serif font-semibold py-3 px-6 rounded-lg transition-all duration-150 tracking-parch"
          >
            Go to Call Page
          </a>
        </div>
      </div>
    );
  }

  return <CallRoom roomId={roomId} />;
}

export default App;
