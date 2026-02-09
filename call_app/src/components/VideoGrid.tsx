import { useState, useEffect, useCallback } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  TouchSensor,
  KeyboardSensor,
  useSensors,
  useSensor,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  arrayMove,
  rectSortingStrategy,
} from '@dnd-kit/sortable';
import { VideoTile } from './VideoTile';
import { SortableVideoTile } from './SortableVideoTile';
import { CallParticipant } from '../hooks/useWebSocket';
import { RemoteStreamInfo } from '../hooks/useRTCConnection';

const LOCAL_TILE_ID = 'local';

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
  const [tileOrder, setTileOrder] = useState<string[]>([LOCAL_TILE_ID]);
  const [activeDragId, setActiveDragId] = useState<string | null>(null);

  const totalParticipants = participants.length + 1; // +1 for local

  // Sync tileOrder when participants join or leave
  useEffect(() => {
    const currentIds = new Set([LOCAL_TILE_ID, ...participants.map((p) => p.id)]);

    setTileOrder((prev) => {
      // Remove IDs that are no longer present
      const filtered = prev.filter((id) => currentIds.has(id));
      // Append any new IDs that aren't in the order yet
      const existing = new Set(filtered);
      const newIds = [...currentIds].filter((id) => !existing.has(id));
      if (newIds.length === 0 && filtered.length === prev.length) {
        return prev; // no change
      }
      return [...filtered, ...newIds];
    });
  }, [participants]);

  // Sensors: pointer (distance 8px), touch (200ms delay), keyboard
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 5 } }),
    useSensor(KeyboardSensor)
  );

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveDragId(event.active.id as string);
  }, []);

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    setActiveDragId(null);
    const { active, over } = event;
    if (over && active.id !== over.id) {
      setTileOrder((prev) => {
        const oldIndex = prev.indexOf(active.id as string);
        const newIndex = prev.indexOf(over.id as string);
        return arrayMove(prev, oldIndex, newIndex);
      });
    }
  }, []);

  const handleDragCancel = useCallback(() => {
    setActiveDragId(null);
  }, []);

  // Determine grid layout based on participant count
  const getGridConfig = () => {
    if (totalParticipants === 1) {
      return {
        container: 'flex items-center justify-center',
        grid: '',
        tileWrapper: 'w-full max-w-4xl',
      };
    } else if (totalParticipants === 2) {
      return {
        container: 'grid gap-3 sm:gap-4 p-3 sm:p-4',
        grid: 'grid-cols-1 sm:grid-cols-2',
        tileWrapper: '',
      };
    } else if (totalParticipants <= 4) {
      return {
        container: 'grid gap-2 sm:gap-3 p-2 sm:p-4',
        grid: 'grid-cols-1 sm:grid-cols-2',
        tileWrapper: '',
      };
    } else if (totalParticipants <= 6) {
      return {
        container: 'grid gap-2 sm:gap-3 p-2 sm:p-3',
        grid: 'grid-cols-2 lg:grid-cols-3',
        tileWrapper: '',
      };
    } else if (totalParticipants <= 9) {
      return {
        container: 'grid gap-2 p-2 sm:p-3',
        grid: 'grid-cols-2 sm:grid-cols-3',
        tileWrapper: '',
      };
    } else {
      return {
        container: 'grid gap-2 p-2',
        grid: 'grid-cols-2 sm:grid-cols-3 xl:grid-cols-4',
        tileWrapper: '',
      };
    }
  };

  // Find remote stream for a participant by matching stream ID
  const getRemoteStream = (participant: CallParticipant): MediaStream | null => {
    const streamInfo = remoteStreams.get(participant.stream_id);
    if (streamInfo) {
      return streamInfo.stream;
    }

    for (const [, info] of remoteStreams) {
      if (info.streamId === participant.stream_id) {
        return info.stream;
      }
    }

    return null;
  };

  // Look up display name for drag overlay
  const getDisplayName = (id: string): string => {
    if (id === LOCAL_TILE_ID) return localDisplayName;
    const participant = participants.find((p) => p.id === id);
    return participant?.display_name ?? '';
  };

  const config = getGridConfig();

  // Special layout for solo participant (no drag needed)
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

  const participantMap = new Map(participants.map((p) => [p.id, p]));

  return (
    <DndContext
      sensors={sensors}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      <SortableContext items={tileOrder} strategy={rectSortingStrategy}>
        <div className={`h-full w-full ${config.container} ${config.grid} content-center`}>
          {tileOrder.map((id) => {
            if (id === LOCAL_TILE_ID) {
              return (
                <SortableVideoTile key={LOCAL_TILE_ID} id={LOCAL_TILE_ID} tileWrapper={config.tileWrapper}>
                  <VideoTile
                    stream={localStream}
                    displayName={localDisplayName}
                    isAudioOn={localIsAudioOn}
                    isVideoOn={localIsVideoOn}
                    isLocal={true}
                  />
                </SortableVideoTile>
              );
            }
            const participant = participantMap.get(id);
            if (!participant) return null;
            return (
              <SortableVideoTile key={id} id={id} tileWrapper={config.tileWrapper}>
                <VideoTile
                  stream={getRemoteStream(participant)}
                  displayName={participant.display_name}
                  isAudioOn={participant.is_audio_on}
                  isVideoOn={participant.is_video_on}
                />
              </SortableVideoTile>
            );
          })}
        </div>
      </SortableContext>

      <DragOverlay>
        {activeDragId ? (
          <div className="rounded-lg bg-parch-dark-blue border border-parch-accent-blue/50 aspect-video flex items-center justify-center shadow-xl shadow-black/40">
            <div className="flex flex-col items-center gap-2">
              <div className="w-12 h-12 rounded-full bg-parch-light-blue/20 border-2 border-parch-gray/40 flex items-center justify-center">
                <span className="text-xl text-parch-accent-blue font-serif font-semibold">
                  {getDisplayName(activeDragId).charAt(0).toUpperCase()}
                </span>
              </div>
              <span className="text-parch-white text-xs font-medium tracking-parch">
                {getDisplayName(activeDragId)}
              </span>
            </div>
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
