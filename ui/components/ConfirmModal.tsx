'use client';

type ConfirmModalProps = {
  isOpen: boolean;
  title?: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  isConfirming?: boolean;
  confirmAction: () => void;
  cancelAction: () => void;
};

export default function ConfirmModal(props: ConfirmModalProps) {
  if (!props.isOpen) {
    return null;
  }

  return (
    <div className="modal modal-open whitespace-normal text-base font-sans" role="dialog" aria-modal="true" aria-labelledby="confirm-modal-title">
      <div className="modal-box">
        <h3 id="confirm-modal-title" className="text-lg font-bold">
          {props.title ?? 'Confirm'}
        </h3>
        <p className="py-4">{props.message}</p>
        <div className="modal-action">
          <button
            type="button"
            className="btn"
            onClick={props.cancelAction}
            disabled={props.isConfirming}
          >
            {props.cancelLabel ?? 'Cancel'}
          </button>
          <button
            type="button"
            className="btn btn-error"
            onClick={props.confirmAction}
            disabled={props.isConfirming}
          >
            {props.isConfirming ? 'Working...' : (props.confirmLabel ?? 'OK')}
          </button>
        </div>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        aria-label="Close confirmation dialog"
        onClick={props.cancelAction}
      />
    </div>
  );
}
