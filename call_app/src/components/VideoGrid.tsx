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
  const getGridClass = () => {
    if (totalParticipants === 1) {
      return 'grid-cols-1';
    } else if (totalParticipants === 2) {
      return 'grid-cols-1 md:grid-cols-2';
    } else if (totalParticipants <= 4) {
      return 'grid-cols-2';
    } else if (totalParticipants <= 6) {
      return 'grid-cols-2 md:grid-cols-3';
    } else {
      return 'grid-cols-2 md:grid-cols-3 lg:grid-cols-4';
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

  return (
    <div className={`grid ${getGridClass()} gap-4 p-4 h-full auto-rows-fr`}>
      {/* Local video tile */}
      <VideoTile
        stream={localStream}
        displayName={localDisplayName}
        isAudioOn={localIsAudioOn}
        isVideoOn={localIsVideoOn}
        isLocal={true}
      />

      {/* Remote participant tiles */}
      {participants.map((participant) => (
        <VideoTile
          key={participant.id}
          stream={getRemoteStream(participant)}
          displayName={participant.display_name}
          isAudioOn={participant.is_audio_on}
          isVideoOn={participant.is_video_on}
        />
      ))}
    </div>
  );
}
