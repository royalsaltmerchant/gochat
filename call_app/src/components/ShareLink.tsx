import { useState } from 'react';

interface ShareLinkProps {
  roomId: string;
  onClose: () => void;
}

export function ShareLink({ roomId, onClose }: ShareLinkProps) {
  const [copied, setCopied] = useState(false);
  const shareUrl = `${window.location.origin}/call/room?id=${roomId}`;

  const copyToClipboard = async () => {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  return (
    <div className="fixed inset-0 bg-parch-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="parch-card rounded-xl p-5 sm:p-6 max-w-md w-full shadow-2xl">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg sm:text-xl font-serif font-semibold text-parch-bright-white tracking-parch">
            Share this call
          </h2>
          <button
            onClick={onClose}
            className="text-parch-gray hover:text-parch-white transition-colors p-1"
          >
            <svg className="w-5 h-5 sm:w-6 sm:h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <p className="text-parch-gray text-sm mb-4 tracking-parch">
          Share this link with others to invite them to the call
        </p>

        <div className="flex items-center gap-2">
          <input
            type="text"
            value={shareUrl}
            readOnly
            className="parch-input flex-1 text-parch-white rounded-lg px-3 sm:px-4 py-2.5 sm:py-3 text-sm tracking-parch"
          />
          <button
            onClick={copyToClipboard}
            className={`parch-btn px-3 sm:px-4 py-2.5 sm:py-3 rounded-lg font-serif font-medium transition-all duration-150 tracking-parch text-sm sm:text-base ${
              copied
                ? 'bg-parch-green text-parch-dark'
                : 'bg-parch-light-blue text-parch-bright-white'
            }`}
          >
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>
      </div>
    </div>
  );
}
