#import <Foundation/Foundation.h>

@interface Animal : NSObject
@property (nonatomic, strong) NSString *name;
@property (nonatomic, strong) NSString *species;
- (void)speak;
- (NSString *)greetWithName:(NSString *)name
                   greeting:(NSString *)greeting;
@end

@protocol Greeter <NSObject>
- (NSString *)greet;
@end

int standalone(int x) {
    return x + 1;
}
