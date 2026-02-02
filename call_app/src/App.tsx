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
        <div className="text-center">
          <h1 className="text-3xl font-bold text-white mb-4">No room specified</h1>
          <p className="text-gray-400 mb-6">
            Please use a valid call link or go back to the landing page
          </p>
          <a
            href="/call"
            className="inline-block bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-xl transition-all duration-200"
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
