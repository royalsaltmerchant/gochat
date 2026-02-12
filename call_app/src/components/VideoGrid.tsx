import { useState, useEffect, useCallback, useRef } from 'react';
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
const GAP = 12;

function computeLayout(
  containerWidth: number,
  containerHeight: number,
  count: number,
): { tileWidth: number; tileHeight: number; cols: number } {
  if (count === 0 || containerWidth <= 0 || containerHeight <= 0) {
    return { tileWidth: 0, tileHeight: 0, cols: 1 };
  }

  let bestArea = 0;
  let bestWidth = 0;
  let bestHeight = 0;
  let bestCols = 1;

  for (let cols = 1; cols <= count; cols++) {
    const rows = Math.ceil(count / cols);
    const availW = containerWidth - GAP * (cols - 1);
    const availH = containerHeight - GAP * (rows - 1);

    let tileW = availW / cols;
    let tileH = tileW * 9 / 16;

    if (tileH * rows > availH) {
      tileH = availH / rows;
      tileW = tileH * 16 / 9;
    }

    const area = tileW * tileH;
    if (area > bestArea) {
      bestArea = area;
      bestWidth = tileW;
      bestHeight = tileH;
      bestCols = cols;
    }
  }

  return { tileWidth: Math.floor(bestWidth), tileHeight: Math.floor(bestHeight), cols: bestCols };
}

interface VideoGridProps {
  localStream: MediaStream | null;
  localDisplayName: string;
  localIsAudioOn: boolean;
  localIsVideoOn: boolean;
  localAudioDeviceId?: string | null;
  participants: CallParticipant[];
  remoteStreams: Map<string, RemoteStreamInfo>;
}

export function VideoGrid({
  localStream,
  localDisplayName,
  localIsAudioOn,
  localIsVideoOn,
  localAudioDeviceId,
  participants,
  remoteStreams,
}: VideoGridProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const [tileOrder, setTileOrder] = useState<string[]>([LOCAL_TILE_ID]);
  const [activeDragId, setActiveDragId] = useState<string | null>(null);

  const totalParticipants = participants.length + 1;

  // Measure container and rebind when the rendered container node changes
  // (e.g. 1-tile layout -> sortable grid).
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    setContainerSize({
      width: el.clientWidth,
      height: el.clientHeight,
    });

    const observer = new ResizeObserver(([entry]) => {
      setContainerSize({
        width: entry.contentRect.width,
        height: entry.contentRect.height,
      });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [totalParticipants]);

  // Sync tileOrder when participants join or leave
  useEffect(() => {
    const currentIds = new Set([LOCAL_TILE_ID, ...participants.map((p) => p.id)]);

    setTileOrder((prev) => {
      const filtered = prev.filter((id) => currentIds.has(id));
      const existing = new Set(filtered);
      const newIds = [...currentIds].filter((id) => !existing.has(id));
      if (newIds.length === 0 && filtered.length === prev.length) {
        return prev;
      }
      return [...filtered, ...newIds];
    });
  }, [participants]);

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

  const getRemoteStream = (participant: CallParticipant): MediaStream | null => {
    const streamInfo = remoteStreams.get(participant.stream_id);
    if (streamInfo) return streamInfo.stream;

    for (const [, info] of remoteStreams) {
      if (info.streamId === participant.stream_id) return info.stream;
    }

    return null;
  };

  const getDisplayName = (id: string): string => {
    if (id === LOCAL_TILE_ID) return localDisplayName;
    const participant = participants.find((p) => p.id === id);
    return participant?.display_name ?? '';
  };

  const { tileWidth, tileHeight, cols } = computeLayout(
    containerSize.width,
    containerSize.height,
    totalParticipants,
  );

  const gridStyle = {
    gridTemplateColumns: `repeat(${cols}, ${tileWidth}px)`,
    gridAutoRows: `${tileHeight}px`,
    gap: GAP,
  };

  if (totalParticipants === 1) {
    return (
      <div ref={containerRef} className="h-full w-full grid place-content-center p-4" style={gridStyle}>
        <VideoTile
          stream={localStream}
          displayName={localDisplayName}
          isAudioOn={localIsAudioOn}
          isVideoOn={localIsVideoOn}
          isLocal={true}
          localAudioDeviceId={localAudioDeviceId}
        />
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
        <div ref={containerRef} className="h-full w-full grid place-content-center p-4" style={gridStyle}>
          {tileOrder.map((id) => {
            if (id === LOCAL_TILE_ID) {
              return (
                <SortableVideoTile key={LOCAL_TILE_ID} id={LOCAL_TILE_ID}>
                  <VideoTile
                    stream={localStream}
                    displayName={localDisplayName}
                    isAudioOn={localIsAudioOn}
                    isVideoOn={localIsVideoOn}
                    isLocal={true}
                    localAudioDeviceId={localAudioDeviceId}
                  />
                </SortableVideoTile>
              );
            }
            const participant = participantMap.get(id);
            if (!participant) return null;
            return (
              <SortableVideoTile key={id} id={id}>
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
          <div
            className="rounded-lg bg-parch-dark-blue border border-parch-accent-blue/50 flex items-center justify-center shadow-xl shadow-black/40"
            style={{ width: tileWidth, height: tileHeight }}
          >
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
