import { useEffect, useRef, useState, useCallback } from 'react';

interface VideoTileProps {
  stream: MediaStream | null;
  displayName: string;
  isAudioOn: boolean;
  isVideoOn: boolean;
  isLocal?: boolean;
  isMuted?: boolean;
}

export function VideoTile({ stream, displayName, isAudioOn, isVideoOn, isLocal = false, isMuted = false }: VideoTileProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  // Volume control only for remote participants (local is always muted to avoid echo)
  const [volume, setVolume] = useState(1.0);
  const [showVolumeSlider, setShowVolumeSlider] = useState(false);

  // Audio level meter for local stream
  const [audioLevel, setAudioLevel] = useState(0);
  const audioContextRef = useRef<AudioContext | null>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const animationFrameRef = useRef<number | null>(null);

  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream;
    }
  }, [stream]);

  // Apply volume to video element (remote only)
  useEffect(() => {
    if (videoRef.current && !isLocal && !isMuted) {
      videoRef.current.volume = volume;
    }
  }, [volume, isLocal, isMuted]);

  // Audio level meter for local stream
  useEffect(() => {
    if (!isLocal || !stream || !isAudioOn) {
      setAudioLevel(0);
      return;
    }

    // Create audio context and analyser
    const audioContext = new AudioContext();
    const analyser = audioContext.createAnalyser();
    analyser.fftSize = 256;
    analyser.smoothingTimeConstant = 0.8;

    const source = audioContext.createMediaStreamSource(stream);
    source.connect(analyser);

    audioContextRef.current = audioContext;
    analyserRef.current = analyser;

    const dataArray = new Uint8Array(analyser.frequencyBinCount);

    const updateLevel = () => {
      if (analyserRef.current) {
        analyserRef.current.getByteFrequencyData(dataArray);
        // Calculate average level
        const average = dataArray.reduce((a, b) => a + b, 0) / dataArray.length;
        // Normalize to 0-1 range
        setAudioLevel(Math.min(average / 128, 1));
      }
      animationFrameRef.current = requestAnimationFrame(updateLevel);
    };

    updateLevel();

    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
      }
      if (audioContextRef.current) {
        audioContextRef.current.close();
      }
    };
  }, [isLocal, stream, isAudioOn]);

  const handleVolumeChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setVolume(parseFloat(e.target.value));
  }, []);

  return (
    <div className="relative bg-parch-dark-blue rounded-lg overflow-hidden h-full border border-parch-gray/30 shadow-lg shadow-black/20">
      {/* Subtle inner glow for depth */}
      <div className="absolute inset-0 rounded-lg shadow-[inset_0_1px_0_0_rgba(157,179,211,0.1)] pointer-events-none z-[1]" />

      {/* Always render video element to preserve srcObject */}
      <video
        ref={videoRef}
        autoPlay
        playsInline
        muted={isLocal || isMuted}
        className={`w-full h-full object-cover ${isLocal ? 'scale-x-[-1]' : ''} ${
          stream && isVideoOn ? 'block' : 'hidden'
        }`}
      />
      {/* Placeholder when video is off */}
      {(!stream || !isVideoOn) && (
        <div className="absolute inset-0 flex items-center justify-center bg-gradient-to-br from-parch-dark-blue to-parch-second-dark">
          <div className="w-16 h-16 sm:w-20 sm:h-20 rounded-full bg-parch-light-blue/20 border-2 border-parch-gray/40 flex items-center justify-center">
            <span className="text-2xl sm:text-3xl text-parch-accent-blue font-serif font-semibold tracking-parch">
              {displayName.charAt(0).toUpperCase()}
            </span>
          </div>
        </div>
      )}

      {/* Name overlay */}
      <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-parch-black/80 via-parch-black/40 to-transparent p-2 sm:p-3">
        <div className="flex items-center justify-between gap-2">
          <span className="text-parch-white text-xs sm:text-sm font-medium truncate tracking-parch">
            {displayName} {isLocal && <span className="text-parch-accent-blue">(You)</span>}
          </span>
          <div className="flex items-center gap-1.5 flex-shrink-0">
            {/* Audio level meter for local */}
            {isLocal && isAudioOn && (
              <div className="flex items-center gap-0.5 h-4" title="Mic level">
                {[0, 1, 2, 3, 4].map((i) => (
                  <div
                    key={i}
                    className={`w-0.5 sm:w-1 rounded-full transition-all duration-75 ${
                      audioLevel > i * 0.2
                        ? i < 3
                          ? 'bg-parch-green'
                          : i < 4
                          ? 'bg-parch-yellow'
                          : 'bg-parch-light-red'
                        : 'bg-parch-gray/50'
                    }`}
                    style={{
                      height: `${40 + i * 15}%`,
                    }}
                  />
                ))}
              </div>
            )}
            {/* Volume control button for remote participants */}
            {!isLocal && !isMuted && (
              <button
                onClick={() => setShowVolumeSlider(!showVolumeSlider)}
                className="bg-parch-dark-blue/80 hover:bg-parch-light-blue/60 rounded p-1 transition-colors border border-parch-gray/30"
                title="Adjust volume"
              >
                {volume === 0 ? (
                  <svg className="w-3.5 h-3.5 text-parch-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
                  </svg>
                ) : volume < 0.5 ? (
                  <svg className="w-3.5 h-3.5 text-parch-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.536 8.464a5 5 0 010 7.072M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  </svg>
                ) : (
                  <svg className="w-3.5 h-3.5 text-parch-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  </svg>
                )}
              </button>
            )}
            {!isAudioOn && (
              <div className="bg-parch-light-red/90 rounded p-1">
                <svg className="w-3.5 h-3.5 text-parch-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
                </svg>
              </div>
            )}
            {!isVideoOn && (
              <div className="bg-parch-light-red/90 rounded p-1">
                <svg className="w-3.5 h-3.5 text-parch-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
                </svg>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Volume control modal overlay (remote only) */}
      {!isLocal && showVolumeSlider && (
        <div
          className="absolute inset-0 bg-parch-black/70 backdrop-blur-sm flex flex-col items-center justify-center rounded-lg z-10"
          onClick={() => setShowVolumeSlider(false)}
        >
          <div
            className="parch-card rounded-lg p-4 sm:p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-3 sm:gap-4 mb-3">
              {/* Speaker icon */}
              <div className="text-parch-accent-blue">
                {volume === 0 ? (
                  <svg className="w-6 h-6 sm:w-8 sm:h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
                  </svg>
                ) : (
                  <svg className="w-6 h-6 sm:w-8 sm:h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  </svg>
                )}
              </div>
              {/* Volume slider */}
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                value={volume}
                onChange={handleVolumeChange}
                className="w-24 sm:w-32 h-2 bg-parch-second-dark rounded appearance-none cursor-pointer accent-parch-light-blue"
              />
              {/* Volume percentage */}
              <span className="text-parch-white text-xs sm:text-sm font-medium w-10 sm:w-12 text-right font-serif">
                {Math.round(volume * 100)}%
              </span>
            </div>
            <p className="text-parch-gray text-xs text-center tracking-parch">
              {displayName}'s volume
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
