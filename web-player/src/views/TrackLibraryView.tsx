import { TrackList } from "../components/media/TrackList";
import type { TrackRowItem } from "../models";

type TrackLibraryViewProps = {
  rows: TrackRowItem[];
  loading: boolean;
  emptyTitle: string;
  emptyDescription: string;
  currentTrackPath?: string;
  isPlaying: boolean;
  onPlayRow: (index: number) => void;
  onToggleCurrent: () => void;
};

export function TrackLibraryView(props: TrackLibraryViewProps) {
  return <TrackList {...props} />;
}
