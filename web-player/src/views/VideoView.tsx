import type { VideoFile } from "../api";
import { VideoList } from "../components/media/VideoList";

type VideoViewProps = {
  items: VideoFile[];
  loading: boolean;
  onPlay: (path: string, title: string) => void;
};

export function VideoView(props: VideoViewProps) {
  return <VideoList {...props} />;
}
