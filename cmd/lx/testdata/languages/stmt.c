/* Statements left at the top level by a broken function are not
   declarations. */
    else
        run_kernel(order, uplo, m, k, alpha,
                   a, lda, b, ldb);
    return check_error();
}

int after(void);
