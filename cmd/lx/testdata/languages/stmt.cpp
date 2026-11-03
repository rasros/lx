// Statements left at the top level by a broken function are not
// declarations.
    } catch (std::invalid_argument& ex) {
        return parse_error(ex);
    }
    else if (mode > 0)
        run_kernel(mode);
    return check_error();
}

int after();
