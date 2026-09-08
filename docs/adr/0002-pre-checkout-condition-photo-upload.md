# Pre-checkout condition photo upload

Campus Vault backend should provide a pre-checkout condition-photo upload endpoint that returns a URL or key usable as `beforeConditionUrl` during Borrow Item checkout. The existing Borrowing image upload endpoint requires a Borrowing ID, but that ID does not exist until checkout succeeds, so a separate pre-checkout upload contract avoids forcing members to paste raw image URLs or making checkout depend on an impossible upload order.
