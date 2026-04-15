#import <Foundation/Foundation.h>

/** Animal with name and species. */
@interface Animal : NSObject
@property (nonatomic, strong) NSString *name;
@property (nonatomic, strong) NSString *species; // taxonomy
- (void)speak;
- (NSString *)greetWithName:(NSString *)name
                   greeting:(NSString *)greeting; // multi-line selector
@end

/* Protocol for greeting behaviour. */
@protocol Greeter <NSObject>
- (NSString *)greet;
@end

// Standalone C function.
int standalone(int x) {
    return x + 1;
}
