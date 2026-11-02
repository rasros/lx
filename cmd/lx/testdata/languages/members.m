#import <Foundation/Foundation.h>

/** A counter with instance variables. */
@interface Counter : NSObject {
    NSUInteger count;
@private
    id secret;
@public
    BOOL enabled;
#ifdef DEBUG
    NSString *debugName;
#endif
}
@property (nonatomic) NSUInteger step;
- (void)increment;
#if TARGET_OS_IPHONE
- (void)vibrate;
#else
- (void)beep;
#endif
+ (instancetype)counterWithStep:(NSUInteger)step;
@end

/* Something that can be counted. */
@protocol Countable <NSObject>
@required
- (NSUInteger)count;
@optional
- (void)reset;
@property (nonatomic) BOOL frozen;
@end
