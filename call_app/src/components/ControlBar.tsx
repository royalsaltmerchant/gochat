import { useState } from 'react';
import type { MediaDeviceInfo } from '../hooks/useLocalStream';

interface ControlBarProps {
  isAudioOn: boolean;
  isVideoOn: boolean;
  onToggleAudio: () => void;
  onToggleVideo: () => void;
  onLeave: () => void;
  onShareLink: () => void;
  audioDevices?: MediaDeviceInfo[];
  videoDevices?: MediaDeviceInfo[];
  selectedAudioDeviceId?: string | null;
  selectedVideoDeviceId?: string | null;
  onSelectAudioDevice?: (deviceId: string) => void | Promise<void>;
  onSelectVideoDevice?: (deviceId: string) => void | Promise<void>;
}

export function ControlBar({
  isAudioOn,
  isVideoOn,
  onToggleAudio,
  onToggleVideo,
  onLeave,
  onShareLink,
  audioDevices = [],
  videoDevices = [],
  selectedAudioDeviceId,
  selectedVideoDeviceId,
  onSelectAudioDevice,
  onSelectVideoDevice,
}: ControlBarProps) {
  const [showSettings, setShowSettings] = useState(false);
  const hasDeviceSettings = audioDevices.length > 0 || videoDevices.length > 0;

  return (
    <div className="relative bg-parch-dark-blue/95 backdrop-blur-sm border-t border-parch-gray/40 px-3 sm:px-4 py-2.5 sm:py-3">
      {showSettings && hasDeviceSettings && (
        <div className="absolute bottom-full mb-2 left-1/2 -translate-x-1/2 w-[min(92vw,420px)] parch-card rounded-lg p-3 sm:p-4 shadow-xl border border-parch-gray/50">
          <h3 className="text-parch-accent-blue text-sm font-medium mb-3 tracking-parch font-serif">
            Device Settings
          </h3>

          {audioDevices.length > 0 && (
            <div className="mb-3">
              <label className="block text-parch-gray text-xs mb-1.5 tracking-parch">
                Microphone
              </label>
              <select
                value={selectedAudioDeviceId || audioDevices[0].deviceId}
                onChange={(e) => {
                  void onSelectAudioDevice?.(e.target.value);
                }}
                className="parch-input w-full text-parch-white text-sm rounded-lg px-3 py-2 tracking-parch bg-parch-dark-blue border border-parch-gray/30 focus:border-parch-light-blue/50 focus:outline-none"
              >
                {audioDevices.map((device) => (
                  <option key={device.deviceId} value={device.deviceId}>
                    {device.label}
                  </option>
                ))}
              </select>
            </div>
          )}

          {videoDevices.length > 0 && (
            <div>
              <label className="block text-parch-gray text-xs mb-1.5 tracking-parch">
                Camera
              </label>
              <select
                value={selectedVideoDeviceId || videoDevices[0].deviceId}
                onChange={(e) => {
                  void onSelectVideoDevice?.(e.target.value);
                }}
                className="parch-input w-full text-parch-white text-sm rounded-lg px-3 py-2 tracking-parch bg-parch-dark-blue border border-parch-gray/30 focus:border-parch-light-blue/50 focus:outline-none"
              >
                {videoDevices.map((device) => (
                  <option key={device.deviceId} value={device.deviceId}>
                    {device.label}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
      )}

      <div className="flex items-center justify-center gap-2 sm:gap-3">
        {/* Mic toggle */}
        <button
          onClick={onToggleAudio}
          className={`parch-btn p-3 sm:p-4 rounded-lg transition-all duration-150 ${
            isAudioOn
              ? 'bg-parch-light-blue text-parch-bright-white hover:brightness-110'
              : 'bg-parch-light-red text-parch-bright-white hover:brightness-110'
          }`}
          title={isAudioOn ? 'Mute microphone' : 'Unmute microphone'}
        >
          {isAudioOn ? (
            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
            </svg>
          ) : (
            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
            </svg>
          )}
        </button>

        {/* Camera toggle */}
        <button
          onClick={onToggleVideo}
          className={`parch-btn p-3 sm:p-4 rounded-lg transition-all duration-150 ${
            isVideoOn
              ? 'bg-parch-light-blue text-parch-bright-white hover:brightness-110'
              : 'bg-parch-light-red text-parch-bright-white hover:brightness-110'
          }`}
          title={isVideoOn ? 'Turn off camera' : 'Turn on camera'}
        >
          {isVideoOn ? (
            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
          ) : (
            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
            </svg>
          )}
        </button>

        {/* Device settings */}
        {hasDeviceSettings && (
          <button
            onClick={() => setShowSettings((prev) => !prev)}
            className={`parch-btn p-3 sm:p-4 rounded-lg transition-all duration-150 ${
              showSettings
                ? 'bg-parch-light-blue text-parch-bright-white'
                : 'bg-parch-second-blue text-parch-bright-white hover:brightness-110'
            }`}
            title="Device settings"
          >
            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
        )}

        {/* Share link */}
        <button
          onClick={onShareLink}
          className="parch-btn p-3 sm:p-4 rounded-lg bg-parch-second-blue text-parch-bright-white transition-all duration-150 hover:brightness-110"
          title="Share link"
        >
          <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
          </svg>
        </button>

        {/* Leave call */}
        <button
          onClick={onLeave}
          className="parch-btn p-3 sm:p-4 rounded-lg bg-parch-dark-red text-parch-bright-white transition-all duration-150 hover:brightness-110"
          title="Leave call"
        >
          <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 8l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2M5 3a2 2 0 00-2 2v1c0 8.284 6.716 15 15 15h1a2 2 0 002-2v-3.28a1 1 0 00-.684-.948l-4.493-1.498a1 1 0 00-1.21.502l-1.13 2.257a11.042 11.042 0 01-5.516-5.517l2.257-1.128a1 1 0 00.502-1.21L9.228 3.683A1 1 0 008.279 3H5z" />
          </svg>
        </button>
      </div>
    </div>
  );
}
