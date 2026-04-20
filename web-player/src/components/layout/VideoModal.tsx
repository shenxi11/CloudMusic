import { X } from "lucide-react";

type VideoModalProps = {
  title: string;
  videoRef: React.RefObject<HTMLVideoElement>;
  onClose: () => void;
};

export function VideoModal({ title, videoRef, onClose }: VideoModalProps) {
  return (
    <div className="video-modal" onClick={onClose}>
      <div className="video-modal-card" onClick={(event) => event.stopPropagation()}>
        <div className="surface-head">
          <div>
            <strong>{title}</strong>
            <small>在线视频播放</small>
          </div>
          <button type="button" className="icon-button subtle" onClick={onClose}>
            <X size={16} />
          </button>
        </div>
        <video ref={videoRef} controls className="video-element" />
      </div>
    </div>
  );
}
