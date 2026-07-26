package goframe

type protectedSubtreeTransaction struct {
	boundary *componentInstance
	parent   *protectedSubtreeTransaction
	attempts []*componentInstance
	failed   bool
	active   bool
}

var currentProtectedSubtreeTransaction *protectedSubtreeTransaction

func runProtectedSubtreeLifecycleTransaction(boundary *componentInstance, reconcile func()) {
	transaction := beginProtectedSubtreeLifecycleTransaction(boundary)
	completed := false
	defer func() {
		finishProtectedSubtreeLifecycleTransaction(transaction, completed)
	}()
	reconcile()
	completed = true
}

func beginProtectedSubtreeLifecycleTransaction(boundary *componentInstance) *protectedSubtreeTransaction {
	if !componentHasProtectedErrorBoundary(boundary) {
		panic("goframe: protected subtree transaction requires an active error boundary")
	}
	transaction := &boundary.errorBoundary.transaction
	if transaction.active {
		panic("goframe: error boundary transaction is already active")
	}
	if len(transaction.attempts) != 0 {
		panic("goframe: error boundary transaction retained lifecycle attempts")
	}
	transaction.boundary = boundary
	transaction.parent = currentProtectedSubtreeTransaction
	transaction.failed = false
	transaction.active = true
	currentProtectedSubtreeTransaction = transaction
	return transaction
}

func finishProtectedSubtreeLifecycleTransaction(
	transaction *protectedSubtreeTransaction,
	completed bool,
) {
	if transaction == nil || !transaction.active {
		panic("goframe: protected subtree transaction is not active")
	}
	if currentProtectedSubtreeTransaction != transaction {
		panic("goframe: protected subtree transaction stack mismatch")
	}

	parent := transaction.parent
	currentProtectedSubtreeTransaction = parent
	transaction.parent = nil
	transaction.active = false

	if !completed || transaction.failed {
		rollbackProtectedSubtreeLifecycleAttempts(transaction.attempts)
		clearProtectedSubtreeLifecycleAttempts(transaction)
		return
	}
	if parent != nil {
		// The outer boundary still owns commit: a later outer failure must be
		// able to roll back work from this successful nested subtree.
		parent.attempts = append(parent.attempts, transaction.attempts...)
		clearProtectedSubtreeLifecycleAttempts(transaction)
		return
	}

	commitProtectedSubtreeLifecycleAttempts(transaction.attempts)
	clearProtectedSubtreeLifecycleAttempts(transaction)
}

func stageProtectedSubtreeLifecycleAttempt(instance *componentInstance) bool {
	transaction := currentProtectedSubtreeTransaction
	if transaction == nil {
		return false
	}
	transaction.attempts = append(transaction.attempts, instance)
	return true
}

func failProtectedSubtreeLifecycleTransaction(boundary *componentInstance) {
	for transaction := currentProtectedSubtreeTransaction; transaction != nil; transaction = transaction.parent {
		if transaction.boundary == boundary {
			transaction.failed = true
			return
		}
	}
}

func componentHasProtectedErrorBoundary(instance *componentInstance) bool {
	return instance != nil &&
		instance.errorBoundary != nil &&
		instance.errorBoundary.phase == errorBoundaryProtected
}

func nearestProtectedErrorBoundary(instance *componentInstance) *componentInstance {
	for current := instance.parent; current != nil; current = current.parent {
		if componentHasProtectedErrorBoundary(current) {
			return current
		}
	}
	return nil
}

func runProtectedDescendantLifecycleTransaction(instance *componentInstance, reconcile func()) {
	if currentProtectedSubtreeTransaction != nil {
		reconcile()
		return
	}
	// Dirty component updates can enter below a boundary without rerendering
	// the boundary component itself, so establish the missing protected scope.
	boundary := nearestProtectedErrorBoundary(instance)
	if boundary == nil {
		reconcile()
		return
	}
	runProtectedSubtreeLifecycleTransaction(boundary, reconcile)
}

func commitProtectedSubtreeLifecycleAttempts(attempts []*componentInstance) {
	next := 0
	defer func() {
		if recovered := recover(); recovered != nil {
			rollbackProtectedSubtreeLifecycleAttempts(attempts[next:])
			panic(recovered)
		}
	}()
	for next < len(attempts) {
		commitLifecycleRenderAttempt(attempts[next])
		next++
	}
}

func rollbackProtectedSubtreeLifecycleAttempts(attempts []*componentInstance) {
	for index := len(attempts) - 1; index >= 0; index-- {
		rollbackLifecycleRenderAttempt(attempts[index])
	}
}

func clearProtectedSubtreeLifecycleAttempts(transaction *protectedSubtreeTransaction) {
	clear(transaction.attempts)
	transaction.attempts = transaction.attempts[:0]
	transaction.boundary = nil
	transaction.failed = false
}
