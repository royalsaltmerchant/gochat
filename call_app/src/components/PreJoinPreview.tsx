import { useEffect, useRef, useState } from 'react';

interface PreJoinPreviewProps {
  localStream: MediaStream | null;
  isAudioOn: boolean;
  isVideoOn: boolean;
  onToggleAudio: () => void;
  onToggleVideo: () => void;
  onJoin: (displayName: string) => void;
  error: string | null;
}

export function PreJoinPreview({
  localStream,
  isAudioOn,
  isVideoOn,
  onToggleAudio,
  onToggleVideo,
  onJoin,
  error,
}: PreJoinPreviewProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [displayName, setDisplayName] = useState('');

  useEffect(() => {
    if (videoRef.current && localStream) {
      videoRef.current.srcObject = localStream;
    }
  }, [localStream]);

  const handleJoin = () => {
    const name = displayName.trim() || 'Guest';
    onJoin(name);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleJoin();
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="max-w-lg w-full">
        <div className="text-center mb-6">
          <h1 className="text-2xl sm:text-3xl font-serif font-bold text-parch-bright-white mb-2 tracking-parch">
            Ready to join?
          </h1>
          <p className="text-parch-gray text-sm sm:text-base tracking-parch">
            Check your camera and microphone before joining
          </p>
        </div>

        <div className="parch-card rounded-xl p-4 sm:p-6 shadow-2xl">
          {/* Video preview */}
          <div className="relative bg-parch-second-dark rounded-lg overflow-hidden aspect-video mb-5 border border-parch-gray/30">
            {/* Inner highlight */}
            <div className="absolute inset-0 rounded-lg shadow-[inset_0_1px_0_0_rgba(157,179,211,0.08)] pointer-events-none z-[1]" />

            {/* Always render video element to preserve srcObject */}
            <video
              ref={videoRef}
              autoPlay
              playsInline
              muted
              className={`w-full h-full object-cover scale-x-[-1] ${
                localStream && isVideoOn ? 'block' : 'hidden'
              }`}
            />
            {/* Placeholder when video is off */}
            {(!localStream || !isVideoOn) && (
              <div className="absolute inset-0 flex items-center justify-center bg-gradient-to-br from-parch-dark-blue to-parch-second-dark">
                <div className="w-20 h-20 sm:w-24 sm:h-24 rounded-full bg-parch-light-blue/15 border-2 border-parch-gray/30 flex items-center justify-center">
                  <svg className="w-10 h-10 sm:w-12 sm:h-12 text-parch-gray" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                  </svg>
                </div>
              </div>
            )}

            {/* Media controls overlay */}
            <div className="absolute bottom-3 sm:bottom-4 left-1/2 transform -translate-x-1/2 flex gap-2 sm:gap-3">
              <button
                onClick={onToggleAudio}
                className={`parch-btn p-2.5 sm:p-3 rounded-lg transition-all duration-150 ${
                  isAudioOn
                    ? 'bg-parch-light-blue/90 text-parch-bright-white'
                    : 'bg-parch-light-red/90 text-parch-bright-white'
                }`}
              >
                {isAudioOn ? (
                  <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
                  </svg>
                ) : (
                  <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
                  </svg>
                )}
              </button>

              <button
                onClick={onToggleVideo}
                className={`parch-btn p-2.5 sm:p-3 rounded-lg transition-all duration-150 ${
                  isVideoOn
                    ? 'bg-parch-light-blue/90 text-parch-bright-white'
                    : 'bg-parch-light-red/90 text-parch-bright-white'
                }`}
              >
                {isVideoOn ? (
                  <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                ) : (
                  <svg className="w-4 h-4 sm:w-5 sm:h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
                  </svg>
                )}
              </button>
            </div>
          </div>

          {error && (
            <div className="bg-parch-light-red/15 border border-parch-light-red/40 text-parch-light-red rounded-lg px-4 py-3 mb-4 text-sm tracking-parch">
              {error}
            </div>
          )}

          {/* Name input */}
          <div className="mb-4">
            <label className="block text-parch-accent-blue text-sm font-medium mb-2 tracking-parch font-serif">
              Your name
            </label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              onKeyPress={handleKeyPress}
              placeholder="Enter your name"
              className="parch-input w-full text-parch-white placeholder-parch-gray/60 rounded-lg px-4 py-3 transition-all tracking-parch"
              autoFocus
            />
          </div>

          {/* Join button */}
          <button
            onClick={handleJoin}
            disabled={!localStream}
            className="parch-btn w-full bg-parch-light-blue disabled:bg-parch-gray disabled:cursor-not-allowed text-parch-bright-white font-serif font-semibold py-3.5 sm:py-4 px-6 rounded-lg transition-all duration-150 tracking-parch text-base sm:text-lg"
          >
            {localStream ? 'Join Call' : 'Setting up...'}
          </button>
        </div>

        {/* Subtle branding */}
        <p className="text-center text-parch-gray/50 text-xs mt-4 tracking-parch font-serif">
          Parch Voice
        </p>
      </div>
    </div>
  );
}
