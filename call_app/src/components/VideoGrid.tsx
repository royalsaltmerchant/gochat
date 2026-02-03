import { VideoTile } from './VideoTile';
import { CallParticipant } from '../hooks/useWebSocket';
import { RemoteStreamInfo } from '../hooks/useRTCConnection';

interface VideoGridProps {
  localStream: MediaStream | null;
  localDisplayName: string;
  localIsAudioOn: boolean;
  localIsVideoOn: boolean;
  participants: CallParticipant[];
  remoteStreams: Map<string, RemoteStreamInfo>;
}

export function VideoGrid({
  localStream,
  localDisplayName,
  localIsAudioOn,
  localIsVideoOn,
  participants,
  remoteStreams,
}: VideoGridProps) {
  const totalParticipants = participants.length + 1; // +1 for local

  // Determine grid layout based on participant count
  // Uses a container-aware approach to prevent overflow
  const getGridConfig = () => {
    if (totalParticipants === 1) {
      // Solo: center a single large tile
      return {
        container: 'flex items-center justify-center',
        grid: '',
        tileWrapper: 'w-full max-w-4xl',
      };
    } else if (totalParticipants === 2) {
      // Duo: side by side on desktop, stacked on mobile
      return {
        container: 'grid gap-3 sm:gap-4 p-3 sm:p-4',
        grid: 'grid-cols-1 sm:grid-cols-2',
        tileWrapper: '',
      };
    } else if (totalParticipants <= 4) {
      // 3-4: 2x2 grid that scales well
      return {
        container: 'grid gap-2 sm:gap-3 p-2 sm:p-4',
        grid: 'grid-cols-1 sm:grid-cols-2',
        tileWrapper: '',
      };
    } else if (totalParticipants <= 6) {
      // 5-6: 2 cols mobile, 3 cols desktop
      return {
        container: 'grid gap-2 sm:gap-3 p-2 sm:p-3',
        grid: 'grid-cols-2 lg:grid-cols-3',
        tileWrapper: '',
      };
    } else if (totalParticipants <= 9) {
      // 7-9: 3x3 max
      return {
        container: 'grid gap-2 p-2 sm:p-3',
        grid: 'grid-cols-2 sm:grid-cols-3',
        tileWrapper: '',
      };
    } else {
      // 10+: 4 cols on large screens
      return {
        container: 'grid gap-2 p-2',
        grid: 'grid-cols-2 sm:grid-cols-3 xl:grid-cols-4',
        tileWrapper: '',
      };
    }
  };

  // Find remote stream for a participant by matching stream ID
  const getRemoteStream = (participant: CallParticipant): MediaStream | null => {
    // Try to match by stream_id
    const streamInfo = remoteStreams.get(participant.stream_id);
    if (streamInfo) {
      return streamInfo.stream;
    }

    // Fallback: iterate and try to find a match
    for (const [, info] of remoteStreams) {
      if (info.streamId === participant.stream_id) {
        return info.stream;
      }
    }

    return null;
  };

  const config = getGridConfig();

  // Special layout for solo participant
  if (totalParticipants === 1) {
    return (
      <div className={`h-full w-full ${config.container} p-4 sm:p-6`}>
        <div className={config.tileWrapper}>
          <VideoTile
            stream={localStream}
            displayName={localDisplayName}
            isAudioOn={localIsAudioOn}
            isVideoOn={localIsVideoOn}
            isLocal={true}
          />
        </div>
      </div>
    );
  }

  return (
    <div className={`h-full w-full ${config.container} ${config.grid} content-center`}>
      {/* Local video tile */}
      <div className={config.tileWrapper}>
        <VideoTile
          stream={localStream}
          displayName={localDisplayName}
          isAudioOn={localIsAudioOn}
          isVideoOn={localIsVideoOn}
          isLocal={true}
        />
      </div>

      {/* Remote participant tiles */}
      {participants.map((participant) => (
        <div key={participant.id} className={config.tileWrapper}>
          <VideoTile
            stream={getRemoteStream(participant)}
            displayName={participant.display_name}
            isAudioOn={participant.is_audio_on}
            isVideoOn={participant.is_video_on}
          />
        </div>
      ))}
    </div>
  );
}
