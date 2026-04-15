;;; Demo utility functions.

;; Greet with a greeting and name.
(defun greet (name greeting)
  (format t "~a, ~a~%" greeting name))

; Simple addition.
(defun add (a b)
  (+ a b))

;; Sum three numbers.
(defun combine (a b c)
  (+ a b c))
