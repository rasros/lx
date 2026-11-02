#import <Foundation/Foundation.h>

/** A counter with instance variables. */
@interface Counter : NSObject {
    NSUInteger count;
@private
    id secret;
@public
    BOOL enabled;
}
@property (nonatomic) NSUInteger step;
- (void)increment;
+ (instancetype)counterWithStep:(NSUInteger)step;
@end
