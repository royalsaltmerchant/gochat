import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { ReactNode } from 'react';

interface SortableVideoTileProps {
  id: string;
  children: ReactNode;
  tileWrapper: string;
}

export function SortableVideoTile({ id, children, tileWrapper }: SortableVideoTileProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : undefined,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`${tileWrapper} cursor-grab active:cursor-grabbing`}
      {...attributes}
      {...listeners}
    >
      {children}
    </div>
  );
}
