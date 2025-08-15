
def keyword(types=()):
    """Decorator to set custom argument types to keywords.

    This decorator creates ``robot_types`` attribute on the decorated
    keyword method or function based on the provided arguments.
    Robot Framework checks them to determine the keyword's
    argument types.

    Types must be given as a dictionary mapping argument names to types or as a list
    (or tuple) of types mapped to arguments based on position. It is OK to
    specify types only to some arguments, and setting ``types`` to ``None``
    disables type conversion altogether.

    Examples::

        @keyword(types={'length': int, 'case_insensitive': bool})
        def types_as_dict(length, case_insensitive=False):
            # ...

        @keyword(types=[int, bool])
        def types_as_list(length, case_insensitive=False):
            # ...

        @keyword(types=None])
        def no_conversion(length, case_insensitive=False):
            # ...

        @keyword
        def func():
            # ...
    """

    def decorator(func):
        func.robot_types = types
        return func
    return decorator
