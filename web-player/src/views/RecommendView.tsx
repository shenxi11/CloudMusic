import type { RecommendationData } from "../api";
import type { RecommendScene } from "../models";
import { RecommendationGrid } from "../components/media/RecommendationGrid";

type RecommendViewProps = {
  data: RecommendationData | null;
  scene: RecommendScene;
  loading: boolean;
  onSceneChange: (scene: RecommendScene) => void;
  onPlay: (index: number) => void;
};

export function RecommendView(props: RecommendViewProps) {
  return <RecommendationGrid {...props} />;
}
